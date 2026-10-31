package scanner

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/KforG/p2pool-scanner-go/config"
	"github.com/KforG/p2pool-scanner-go/geo"
	"github.com/KforG/p2pool-scanner-go/logging"
	"github.com/KforG/p2pool-scanner-go/node"
	"github.com/KforG/p2pool-scanner-go/p2pgo"
)

// maxProbes limits how many nodes are probed at the same time.
const maxProbes = 16

// ScanOnce crawls the network once and adds every node that answers to l.
// It starts from the bootstrap nodes and follows each answering node's peers
// until no new node answers or ctx ends.
func ScanOnce(ctx context.Context, l *NodeList) {
	scanLegacy(ctx, l)
	scanP2PoolGo(ctx, l)
}

// maxP2PoolGoPeers caps how many p2pool-go peers one scan contacts.
const maxP2PoolGoPeers = 500

// scanP2PoolGo finds p2pool-go peers through libp2p peer exchange and adds
// those whose web server answers. Most peers are OCM light nodes whose web
// server only listens on loopback; they don't answer and aren't public nodes.
func scanP2PoolGo(ctx context.Context, l *NodeList) {
	cfg := config.Active.P2PoolGo
	if len(cfg.BootstrapPeers) == 0 {
		return
	}
	peers, err := p2pgo.Discover(ctx, cfg.BootstrapPeers, maxP2PoolGoPeers)
	if err != nil {
		logging.Errorf("p2pool-go scan: %v\n", err)
		return
	}
	logging.Infof("Reached %d p2pool-go peers\n", len(peers))

	seen := map[string]bool{}
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxProbes)
	for _, p := range peers {
		addr := net.JoinHostPort(p.IP, cfg.WebPort)
		if p.IP == "" || seen[addr] {
			continue
		}
		seen[addr] = true
		wg.Add(1)
		sem <- struct{}{}
		go func(addr string) {
			defer wg.Done()
			defer func() { <-sem }()
			var n Node
			start := time.Now()
			if err := node.GetLocalStatsCtx(ctx, addr, &n.LocalStats); err != nil {
				return
			}
			n.ResponseMs = time.Since(start).Milliseconds()
			n.IP = addr
			n.Network = "p2pool-go"
			node.CheckForDomain(n.IP, &n.DomainName)
			l.Add(n)
		}(addr)
	}
	wg.Wait()
}

// target is an address to probe, with the domain it came from if any.
type target struct {
	addr   string // ip:port
	domain string
}

func scanLegacy(ctx context.Context, l *NodeList) {
	seen := map[string]bool{}
	var wave []target
	for _, b := range config.Active.BootstrapNodes {
		host, port, err := net.SplitHostPort(b)
		if err != nil {
			logging.Errorf("Bootstrap node %s is not host:port\n", b)
			continue
		}
		ip, err := resolve(ctx, host)
		if err != nil {
			logging.Infof("%s couldn't be resolved: %v\n", b, err)
			continue
		}
		t := target{addr: net.JoinHostPort(ip, port), domain: b}
		if !seen[t.addr] {
			seen[t.addr] = true
			wave = append(wave, t)
		}
	}

	for len(wave) > 0 && ctx.Err() == nil {
		var mu sync.Mutex
		var next []target
		var wg sync.WaitGroup
		sem := make(chan struct{}, maxProbes)
		for _, t := range wave {
			wg.Add(1)
			sem <- struct{}{}
			go func(t target) {
				defer wg.Done()
				defer func() { <-sem }()
				n, peers, ok := probeLegacy(ctx, t)
				if !ok {
					return
				}
				l.Add(n)
				mu.Lock()
				defer mu.Unlock()
				for _, p := range peers {
					addr := net.JoinHostPort(p, config.Active.Port)
					if !seen[addr] {
						seen[addr] = true
						next = append(next, target{addr: addr})
					}
				}
			}(t)
		}
		wg.Wait()
		wave = next
	}
}

// probeLegacy fetches a legacy node's stats and peers. ok is false if the
// node didn't answer /local_stats.
func probeLegacy(ctx context.Context, t target) (n Node, peers []string, ok bool) {
	start := time.Now()
	if err := node.GetLocalStatsCtx(ctx, t.addr, &n.LocalStats); err != nil {
		return n, nil, false
	}
	n.ResponseMs = time.Since(start).Milliseconds()
	n.IP = t.addr
	n.Network = "legacy"
	n.DomainName = t.domain
	if n.DomainName == "" {
		node.CheckForDomain(n.IP, &n.DomainName)
	}
	_ = node.GetGlobalStatsCtx(ctx, t.addr, &n.GlobalStats)
	if geoEnabled() {
		if err := geo.GetGeoLocation(t.addr, &n.GeoLocation); err != nil {
			logging.Errorf("Failed to get Geolocation of %s\n", t.addr)
		}
	}
	peers, _ = node.GetPeersCtx(ctx, t.addr)
	return n, peers, true
}

// resolve returns host's first IPv4 address, or its first address if it has
// no IPv4 one.
func resolve(ctx context.Context, host string) (string, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		if a.IP.To4() != nil {
			return a.IP.String(), nil
		}
	}
	return addrs[0].IP.String(), nil
}

// geoEnabled is true when a real geolocation access key is configured.
func geoEnabled() bool {
	key := config.Active.GeoLocation.AcessKey
	return key != "" && !strings.Contains(key, "YOUR_IPSTACK_KEY")
}
