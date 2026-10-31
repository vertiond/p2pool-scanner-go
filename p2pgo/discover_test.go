package p2pgo

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// fakeNode is a libp2p host on 127.0.0.1 that answers peer exchange with
// whatever reply returns.
func fakeNode(t *testing.T, reply func() []byte) host.Host {
	t.Helper()
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	h.SetStreamHandler(protocol.ID(PeerExProtocol), func(s network.Stream) {
		defer s.Close()
		_, _ = s.Write(reply())
	})
	return h
}

func peerList(addrs ...string) func() []byte {
	return func() []byte {
		b, _ := json.Marshal(addrs)
		return b
	}
}

func fullAddr(h host.Host) string {
	return fmt.Sprintf("%s/p2p/%s", h.Addrs()[0], h.ID())
}

func ids(peers []Peer) []string {
	var out []string
	for _, p := range peers {
		out = append(out, p.ID)
	}
	sort.Strings(out)
	return out
}

func sorted(s ...string) []string {
	sort.Strings(s)
	return s
}

func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return c
}

func TestDiscoverFollowsPeerExchange(t *testing.T) {
	b := fakeNode(t, peerList())
	a := fakeNode(t, peerList(fullAddr(b)))

	got, err := Discover(ctx(t), []string{fullAddr(a)}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if want := sorted(a.ID().String(), b.ID().String()); fmt.Sprint(ids(got)) != fmt.Sprint(want) {
		t.Fatalf("found %v, want %v", ids(got), want)
	}
	for _, p := range got {
		if p.IP != "127.0.0.1" {
			t.Errorf("%s: IP %q, want 127.0.0.1", p.ID, p.IP)
		}
	}
}

func TestDiscoverSkipsBadPeerList(t *testing.T) {
	b := fakeNode(t, peerList())
	a := fakeNode(t, func() []byte { return []byte("not json") })
	c := fakeNode(t, peerList("/ip4/garbage", fullAddr(b)))

	got, err := Discover(ctx(t), []string{fullAddr(a), fullAddr(c)}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if want := sorted(a.ID().String(), b.ID().String(), c.ID().String()); fmt.Sprint(ids(got)) != fmt.Sprint(want) {
		t.Fatalf("found %v, want %v", ids(got), want)
	}
}

func TestDiscoverUnreachableBootstrap(t *testing.T) {
	priv, _, _ := crypto.GenerateEd25519Key(rand.Reader)
	id, _ := peer.IDFromPrivateKey(priv)
	start := time.Now()
	got, err := Discover(ctx(t), []string{"/ip4/127.0.0.1/tcp/1/p2p/" + id.String()}, 10)
	if err == nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want no peers and an error", got, err)
	}
	if took := time.Since(start); took > 15*time.Second {
		t.Fatalf("took %v", took)
	}
}

func TestDiscoverLimit(t *testing.T) {
	b := fakeNode(t, peerList())
	c := fakeNode(t, peerList())
	a := fakeNode(t, peerList(fullAddr(b), fullAddr(c)))

	got, err := Discover(ctx(t), []string{fullAddr(a)}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("found %d peers, want the limit of 2", len(got))
	}
}

// A reply bigger than the cap is not read in full.
func TestDiscoverCapsReply(t *testing.T) {
	a := fakeNode(t, func() []byte {
		b := []byte(`["`)
		b = append(b, make([]byte, 2*maxReply)...)
		return b
	})
	got, err := Discover(ctx(t), []string{fullAddr(a)}, 10)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v, %v; want just the bootstrap peer", got, err)
	}
}
