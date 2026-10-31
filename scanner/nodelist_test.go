package scanner

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"
)

func ips(l *NodeList) []string {
	var out []string
	for _, n := range l.Snapshot() {
		out = append(out, n.IP)
	}
	sort.Strings(out)
	return out
}

// Dropping two neighbours in one pass used to skip the node swapped into the
// first one's place.
func TestNodeListRetainVisitsEveryNode(t *testing.T) {
	l := &NodeList{}
	for _, ip := range []string{"a", "b", "c", "d"} {
		l.Add(Node{IP: ip})
	}
	calls := 0
	l.Retain(func(n Node) bool {
		calls++
		return n.IP != "a" && n.IP != "b"
	})
	if got := ips(l); !reflect.DeepEqual(got, []string{"c", "d"}) {
		t.Fatalf("kept %v, want [c d]", got)
	}
	if calls != 4 {
		t.Fatalf("keep called %d times, want 4", calls)
	}
}

func TestNodeListAddDedupes(t *testing.T) {
	l := &NodeList{}
	if !l.Add(Node{IP: "a"}) || l.Add(Node{IP: "a"}) {
		t.Fatal("second Add of the same IP should return false")
	}
	if !l.Has("a") || l.Has("b") || len(l.Snapshot()) != 1 {
		t.Fatalf("list = %v", ips(l))
	}
}

func TestNodeListConcurrent(t *testing.T) {
	l := &NodeList{}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = l.Snapshot()
			}
		}
	}()
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			l.Add(Node{IP: fmt.Sprintf("10.0.0.%d", i)})
		}(i)
	}
	wg.Wait()
	close(stop)
	if n := len(l.Snapshot()); n != 50 {
		t.Fatalf("got %d nodes, want 50", n)
	}
}
