package scanner

import "sync"

// NodeList is the set of known nodes, safe to use from several goroutines.
type NodeList struct {
	mu    sync.Mutex
	nodes []Node
}

// Add adds n unless a node with the same IP is already known, and reports
// whether it was added.
func (l *NodeList) Add(n Node) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range l.nodes {
		if k.IP == n.IP {
			return false
		}
	}
	l.nodes = append(l.nodes, n)
	return true
}

// Has reports whether a node with this IP is known.
func (l *NodeList) Has(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range l.nodes {
		if k.IP == ip {
			return true
		}
	}
	return false
}

// Update replaces the known node with n's IP.
func (l *NodeList) Update(n Node) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.nodes {
		if l.nodes[i].IP == n.IP {
			l.nodes[i] = n
			return
		}
	}
}

// Retain drops every node for which keep returns false, calling keep once
// per node. keep runs with the list locked, so it must not block.
func (l *NodeList) Retain(keep func(Node) bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := l.nodes[:0]
	for _, n := range l.nodes {
		if keep(n) {
			kept = append(kept, n)
		}
	}
	l.nodes = kept
}

// Snapshot returns a copy of the known nodes.
func (l *NodeList) Snapshot() []Node {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Node(nil), l.nodes...)
}
