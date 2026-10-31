package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/KforG/p2pool-scanner-go/node"
)

// minerServer is a legacy node whose miner list shrinks from two miners to
// one after the first /local_stats answer.
func minerServer(t *testing.T) string {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/local_stats":
			if atomic.AddInt32(&calls, 1) == 1 {
				fmt.Fprint(w, `{"miner_hash_rates":{"Va":1,"Vb":1}}`)
			} else {
				fmt.Fprint(w, `{"miner_hash_rates":{"Va":1}}`)
			}
		case "/peer_addresses":
			fmt.Fprint(w, `""`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

// A miner that left must not stay in the node's stats.
func TestRescanDropsDepartedMiners(t *testing.T) {
	l := &NodeList{}
	l.Add(Node{IP: minerServer(t)})
	rescan(l)
	rescan(l)
	if got := len(l.Snapshot()[0].LocalStats.MinerHashRates); got != 1 {
		t.Fatalf("node lists %d miners after one left, want 1", got)
	}
}

// The /nodes handler encodes snapshots while the rescan updates the same
// nodes; run with -race.
func TestRescanConcurrentWithSnapshot(t *testing.T) {
	l := &NodeList{}
	l.Add(Node{IP: minerServer(t), LocalStats: node.NodeStats{MinerHashRates: map[string]float64{"x": 1}}})
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				_ = json.NewEncoder(io.Discard).Encode(l.Snapshot())
			}
		}
	}()
	for i := 0; i < 20; i++ {
		rescan(l)
	}
	close(stop)
	<-done
}
