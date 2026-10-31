package node

import (
	"encoding/json"
	"testing"
)

// miner_hash_rates is an object of address to hashrate; synced is only
// reported by p2pool-go nodes.
func TestNodeStatsDecodesMinerRates(t *testing.T) {
	var s NodeStats
	if err := json.Unmarshal([]byte(`{"miner_hash_rates":{"Va":1.5,"Vb":2},"synced":false}`), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.MinerHashRates) != 2 || s.MinerHashRates["Va"] != 1.5 {
		t.Fatalf("MinerHashRates = %v", s.MinerHashRates)
	}
	if s.Synced == nil || *s.Synced {
		t.Fatalf("Synced = %v, want false", s.Synced)
	}

	var legacy NodeStats
	if err := json.Unmarshal([]byte(`{"miner_hash_rates":{}}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Synced != nil {
		t.Fatal("a node that doesn't report synced should leave it nil")
	}
}
