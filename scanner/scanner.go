package scanner

import (
	"fmt"
	"time"

	"github.com/KforG/p2pool-scanner-go/config"
	"github.com/KforG/p2pool-scanner-go/geo"
	"github.com/KforG/p2pool-scanner-go/logging"
	"github.com/KforG/p2pool-scanner-go/node"
)

type Node struct {
	IP          string           `json:"ip"`
	DomainName  string           `json:"domain_name"`
	Network     string           `json:"network"`     // "legacy" or "p2pool-go"
	ResponseMs  int64            `json:"response_ms"` // how long /local_stats took to answer
	LocalStats  node.NodeStats   `json:"node_stats"`
	GlobalStats node.GlobalStats `json:"global_stats"`
	GeoLocation geo.Geo          `json:"geo_location"`
}

func Scanner(l *NodeList) {
	go node.GetAllDomainIPs()
	loadBootstrapNodes(l)

	for {
		logging.Infof("Scanning P2pool network for public P2Pool nodes...\n")
		rescan(l)

		if config.Active.RescanTime > 0 {
			logging.Infof("Updating nodes stats again and rescanning for new peers in %d minutes", config.Active.RescanTime)
			time.Sleep(time.Duration(config.Active.RescanTime) * time.Minute)
		}
	}
}

// rescan refreshes every known node's stats, drops the ones that no longer
// answer and looks for new nodes among their peers.
func rescan(l *NodeList) {
	dead := map[string]bool{}
	for _, n := range l.Snapshot() {
		// Decode into fresh values: a snapshot shares its maps and slices with
		// the stored node, and decoding into them would edit those in place
		// (racing /nodes) and keep miners that have left.
		var local node.NodeStats
		err := node.GetLocalStats(n.IP, &local)
		if err != nil {
			logging.Infof("%s, did not return updated node state, might be unreachable. Removing from node list.\n", n.IP)
			dead[n.IP] = true
			continue
		}
		n.LocalStats = local

		peers, _ := node.GetPeers(n.IP)
		go discoverNewNodes(l, peers)

		var global node.GlobalStats
		if node.GetGlobalStats(n.IP, &global) == nil {
			n.GlobalStats = global
		}
		l.Update(n)
	}
	l.Retain(func(n Node) bool { return !dead[n.IP] })
}

func loadBootstrapNodes(l *NodeList) {
	logging.Infof("Loading bootstrap nodes..")
	for i := 0; i < len(config.Active.BootstrapNodes); i++ {
		var bn Node
		err := node.GetLocalStats(config.Active.BootstrapNodes[i], &bn.LocalStats)
		if err != nil {
			logging.Infof("%s couldn't be reached, consider updating bootstrap nodes\n", config.Active.BootstrapNodes[i])
			continue
		}
		err = node.GetGlobalStats(config.Active.BootstrapNodes[i], &bn.GlobalStats)
		if err != nil {
			logging.Errorf("Unable to fetch GlobalStats from %s\n", config.Active.BootstrapNodes[i])
		}

		bn.DomainName = config.Active.BootstrapNodes[i]
		node.GetIPFromDomain(bn.DomainName, &bn.IP)

		err = geo.GetGeoLocation(bn.IP, &bn.GeoLocation)
		if err != nil {
			logging.Errorf("Unable to fetch geolocation of %s:%s\n", bn.DomainName, bn.IP)
		}
		l.Add(bn)
	}
}

func discoverNewNodes(l *NodeList, peers []string) {
	for i := 0; i < len(peers); i++ {
		addr := fmt.Sprintf(peers[i] + ":" + config.Active.Port)
		if l.Has(addr) {
			// peer is the same as an already known node
			continue
		}
		var an Node

		err := node.GetLocalStats(addr, &an.LocalStats)
		if err != nil {
			// If the peer doesn't respond to this request, it is assumed inactive/private
			continue
		}

		err = node.GetGlobalStats(addr, &an.GlobalStats)
		if err != nil {
			logging.Errorf("Failed to fetch Global stats from %s, node seems to be alive\n", addr, err)
		}

		err = geo.GetGeoLocation(peers[i], &an.GeoLocation)
		if err != nil {
			logging.Errorf("Failed to get Geolocation of %s\n", peers[i])
		}

		an.IP = addr
		node.CheckForDomain(an.IP, &an.DomainName)

		// Add skips the node if another goroutine added it meanwhile.
		l.Add(an)
	}
}
