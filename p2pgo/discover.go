// Package p2pgo finds p2pool-go nodes. p2pool-go nodes talk over libp2p
// and answer peer exchange: a stream on PeerExProtocol replies with a JSON
// array of the multiaddrs of their public peers.
package p2pgo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	manet "github.com/multiformats/go-multiaddr/net"

	"github.com/KforG/p2pool-scanner-go/logging"
)

// PeerExProtocol is p2pool-go's peer exchange protocol.
const PeerExProtocol = "/p2pool/peers/1.0.0"

const (
	maxReply    = 64 << 10 // largest peer list read from one node
	maxDials    = 16       // peers contacted at the same time
	peerTimeout = 10 * time.Second
)

// Peer is a p2pool-go node that answered a connection.
type Peer struct {
	ID string
	IP string // from the connection, so a DNS bootstrap address gets its IP
}

// Discover crawls p2pool-go's peer exchange breadth-first from bootstrap
// (multiaddrs ending in /p2p/<peer id>) and returns every peer it could
// connect to, bootstrap peers included. It stops after limit peers or when
// ctx ends. The error is set only when no bootstrap peer could be reached.
func Discover(ctx context.Context, bootstrap []string, limit int) ([]Peer, error) {
	h, err := libp2p.New(libp2p.NoListenAddrs, libp2p.Transport(tcp.NewTCPTransport))
	if err != nil {
		return nil, err
	}
	defer h.Close()

	seen := map[peer.ID]bool{}
	queue := func(wave []peer.AddrInfo, addr string) []peer.AddrInfo {
		info, err := peer.AddrInfoFromString(addr)
		if err != nil || seen[info.ID] {
			return wave
		}
		seen[info.ID] = true
		return append(wave, *info)
	}

	var wave []peer.AddrInfo
	for _, b := range bootstrap {
		wave = queue(wave, b)
	}

	var found []Peer
	for len(wave) > 0 && len(found) < limit && ctx.Err() == nil {
		var mu sync.Mutex
		var next []peer.AddrInfo
		var wg sync.WaitGroup
		sem := make(chan struct{}, maxDials)
		for _, info := range wave {
			wg.Add(1)
			sem <- struct{}{}
			go func(info peer.AddrInfo) {
				defer wg.Done()
				defer func() { <-sem }()
				p, addrs, ok := visit(ctx, h, info)
				if !ok {
					return
				}
				mu.Lock()
				defer mu.Unlock()
				if len(found) < limit {
					found = append(found, p)
				}
				for _, a := range addrs {
					next = queue(next, a)
				}
			}(info)
		}
		wg.Wait()
		wave = next
	}

	if len(found) == 0 && len(bootstrap) > 0 {
		return nil, fmt.Errorf("no p2pool-go bootstrap peer could be reached")
	}
	return found, nil
}

// visit connects to a peer and asks for its peers. ok is false if the peer
// couldn't be reached; a peer that is reachable but sends no usable peer list
// still counts.
func visit(ctx context.Context, h host.Host, info peer.AddrInfo) (p Peer, addrs []string, ok bool) {
	c, cancel := context.WithTimeout(ctx, peerTimeout)
	defer cancel()
	if err := h.Connect(c, info); err != nil {
		logging.Infof("p2pool-go peer %s unreachable: %v\n", info.ID, err)
		return p, nil, false
	}
	defer h.Network().ClosePeer(info.ID)

	p.ID = info.ID.String()
	for _, conn := range h.Network().ConnsToPeer(info.ID) {
		if ip, err := manet.ToIP(conn.RemoteMultiaddr()); err == nil {
			p.IP = ip.String()
			break
		}
	}

	s, err := h.NewStream(c, info.ID, protocol.ID(PeerExProtocol))
	if err != nil {
		return p, nil, true
	}
	defer s.Close()
	_ = s.SetDeadline(time.Now().Add(peerTimeout))
	if err := json.NewDecoder(io.LimitReader(s, maxReply)).Decode(&addrs); err != nil {
		logging.Infof("p2pool-go peer %s sent no usable peer list: %v\n", info.ID, err)
		return p, nil, true
	}
	return p, addrs, true
}
