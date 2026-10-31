package scanner

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/KforG/p2pool-scanner-go/config"
)

// fakeNode serves handler on ip:port and stops it when the test ends.
func fakeNode(t *testing.T, ip string, port int, handler http.HandlerFunc) {
	t.Helper()
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		t.Fatalf("listen %s:%d: %v", ip, port, err)
	}
	srv := httptest.NewUnstartedServer(handler)
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func withConfig(t *testing.T, c func(*configStruct)) {
	t.Helper()
	old := config.Active
	t.Cleanup(func() { config.Active = old })
	c(&config.Active)
}

type configStruct = config.Config

func TestScanOnceCrawlsLegacyPeers(t *testing.T) {
	port := freePort(t)
	fakeNode(t, "127.0.0.1", port, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/local_stats":
			fmt.Fprint(w, `{"fee":0,"protocol_version":3501,"miner_hash_rates":{"Va":1}}`)
		case "/peer_addresses":
			fmt.Fprint(w, `"127.0.0.2:40000 127.0.0.3"`)
		default:
			fmt.Fprint(w, `{}`)
		}
	})
	fakeNode(t, "127.0.0.2", port, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/peer_addresses":
			fmt.Fprint(w, `""`)
		default:
			fmt.Fprint(w, `{"fee":1}`)
		}
	})
	fakeNode(t, "127.0.0.3", port, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	})
	withConfig(t, func(c *configStruct) {
		c.BootstrapNodes = []string{fmt.Sprintf("127.0.0.1:%d", port)}
		c.Port = fmt.Sprint(port)
	})

	l := &NodeList{}
	ScanOnce(context.Background(), l)

	want := []string{fmt.Sprintf("127.0.0.1:%d", port), fmt.Sprintf("127.0.0.2:%d", port)}
	if got := ips(l); !reflect.DeepEqual(got, want) {
		t.Fatalf("found %v, want %v", got, want)
	}
	for _, n := range l.Snapshot() {
		if n.Network != "legacy" || n.ResponseMs < 0 {
			t.Errorf("%s: network %q, response %d ms", n.IP, n.Network, n.ResponseMs)
		}
		if n.IP == want[0] && len(n.LocalStats.MinerHashRates) != 1 {
			t.Errorf("%s: miner_hash_rates = %v", n.IP, n.LocalStats.MinerHashRates)
		}
	}
}

func TestScanOnceStopsAtDeadline(t *testing.T) {
	port := freePort(t)
	fakeNode(t, "127.0.0.1", port, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		fmt.Fprint(w, `{}`)
	})
	withConfig(t, func(c *configStruct) {
		c.BootstrapNodes = []string{fmt.Sprintf("127.0.0.1:%d", port)}
		c.Port = fmt.Sprint(port)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	ScanOnce(ctx, &NodeList{})
	if took := time.Since(start); took > time.Second {
		t.Fatalf("ScanOnce took %v after a 200ms deadline", took)
	}
}
