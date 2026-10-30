package minimetriccounter

import (
	"math"
	"runtime"
	"sync"
	"testing"
)

const floatTolerance = 1e-9

func approxEq(a, b float64) bool {
	return math.Abs(a-b) < floatTolerance
}

func TestCounterInc(t *testing.T) {
	c := &Counter{}
	c.Inc()
	c.Inc()
	if got := c.Get(); got != 2 {
		t.Fatalf("Get() = %d, want 2", got)
	}
}

func TestCounterAddPositive(t *testing.T) {
	c := &Counter{}
	if !c.Add(10) {
		t.Fatal("Add(10) returned false")
	}
	if got := c.Get(); got != 10 {
		t.Fatalf("Get() = %d, want 10", got)
	}
}

func TestCounterAddNegativeRejected(t *testing.T) {
	c := &Counter{}
	c.Inc()
	if c.Add(-5) {
		t.Fatal("Add(-5) should return false for a counter")
	}
	if got := c.Get(); got != 1 {
		t.Fatalf("value changed after rejected Add; got %d, want 1", got)
	}
}

func TestCounterSnapshotIsCopy(t *testing.T) {
	c := &Counter{}
	c.Add(7)
	snap := c.Snapshot()
	c.Inc()
	if snap != 7 {
		t.Fatalf("snapshot mutated; got %d, want 7", snap)
	}
}

func TestGaugeSet(t *testing.T) {
	g := &Gauge{}
	g.Set(42.5)
	if !approxEq(g.Get(), 42.5) {
		t.Fatalf("Get() = %v, want 42.5", g.Get())
	}
}

func TestGaugeAdd(t *testing.T) {
	g := &Gauge{}
	g.Set(10.0)
	g.Add(5.0)
	if !approxEq(g.Get(), 15.0) {
		t.Fatalf("Get() = %v, want 15.0", g.Get())
	}
}

func TestGaugeSub(t *testing.T) {
	g := &Gauge{}
	g.Set(10.0)
	g.Sub(3.0)
	if !approxEq(g.Get(), 7.0) {
		t.Fatalf("Get() = %v, want 7.0", g.Get())
	}
}

func TestGaugeNegativeValue(t *testing.T) {
	g := &Gauge{}
	g.Set(-1.5)
	if !approxEq(g.Get(), -1.5) {
		t.Fatalf("Get() = %v, want -1.5", g.Get())
	}
}

func TestCounterConcurrentInc(t *testing.T) {
	c := &Counter{}
	const goroutines = 100
	const perG = 1000
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				c.Inc()
			}
		}()
	}
	wg.Wait()
	want := uint64(goroutines * perG)
	if got := c.Get(); got != want {
		t.Fatalf("after concurrent Inc: got %d, want %d", got, want)
	}
}

func TestGaugeConcurrentAdd(t *testing.T) {
	g := &Gauge{}
	const goroutines = 100
	const perG = 1000
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				g.Add(1.0)
			}
		}()
	}
	wg.Wait()
	want := float64(goroutines * perG)
	if !approxEq(g.Get(), want) {
		t.Fatalf("after concurrent Add: got %v, want %v", g.Get(), want)
	}
}

func TestRegistryCounterSameInstance(t *testing.T) {
	r := NewRegistry()
	a := r.Counter("requests")
	b := r.Counter("requests")
	if a != b {
		t.Fatal("registry returned different Counter instances for the same name")
	}
	a.Inc()
	if b.Get() != 1 {
		t.Fatalf("mutation via first instance not visible via second; got %d, want 1", b.Get())
	}
}

func TestRegistryGaugeSameInstance(t *testing.T) {
	r := NewRegistry()
	a := r.Gauge("memory")
	b := r.Gauge("memory")
	if a != b {
		t.Fatal("registry returned different Gauge instances for the same name")
	}
	a.Set(99.0)
	if !approxEq(b.Get(), 99.0) {
		t.Fatalf("mutation via first instance not visible via second; got %v, want 99.0", b.Get())
	}
}

func TestRegistryCounterAndGaugeSeparate(t *testing.T) {
	r := NewRegistry()
	r.Counter("thing").Inc()
	r.Gauge("thing").Set(5.0)
	if r.Counter("thing").Get() != 1 {
		t.Fatal("counter and gauge namespaces should be independent")
	}
	if !approxEq(r.Gauge("thing").Get(), 5.0) {
		t.Fatal("counter and gauge namespaces should be independent")
	}
}

// runOtherGoroutine ensures the lock-free hot path does not require callers
// to stay on one goroutine. This guards against accidental use of
// goroutine-local storage (which none of the types use, but it is cheap to
// verify and catches regressions if the implementation ever changes).
func runOtherGoroutine(fn func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	<-done
	runtime.Gosched()
}

func TestCounterCrossGoroutine(t *testing.T) {
	c := &Counter{}
	runOtherGoroutine(func() { c.Add(3) })
	if c.Get() != 3 {
		t.Fatalf("cross-goroutine Add failed; got %d, want 3", c.Get())
	}
}

func TestGaugeCrossGoroutine(t *testing.T) {
	g := &Gauge{}
	runOtherGoroutine(func() { g.Set(3.14) })
	if !approxEq(g.Get(), 3.14) {
		t.Fatalf("cross-goroutine Set failed; got %v, want 3.14", g.Get())
	}
}
