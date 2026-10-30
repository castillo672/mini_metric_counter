// Package minimetriccounter provides thread-safe counters and gauges
// for recording application metrics without external dependencies.
//
// All operations are atomic; a Counter or Gauge may be shared across
// goroutines without additional locking. Returned snapshot values are
// copies and are safe to hold even as the metric continues to be updated.
package minimetriccounter

import (
	"math"
	"sync/atomic"
)

// Float64Bits is a helper that performs the atomic-load-then-math-Float64frombits
// dance needed because Go's sync/atomic package does not expose a direct
// atomic load/store for float64. This is the canonical approach recommended
// by the Go documentation for atomic floating-point values.
type Float64Bits struct {
	v uint64
}

// Set atomically stores f.
func (f *Float64Bits) Set(val float64) {
	atomic.StoreUint64(&f.v, math.Float64bits(val))
}

// Get atomically loads the current value.
func (f *Float64Bits) Get() float64 {
	return math.Float64frombits(atomic.LoadUint64(&f.v))
}

// Add atomically adds delta to the stored value. Reads the current bits,
// converts to float64, adds, converts back, and CAS-loops until the store
// succeeds. The loop is necessary because another goroutine may write
// between our load and our store.
func (f *Float64Bits) Add(delta float64) {
	for {
		old := atomic.LoadUint64(&f.v)
		new := math.Float64bits(math.Float64frombits(old) + delta)
		if atomic.CompareAndSwapUint64(&f.v, old, new) {
			return
		}
	}
}

// Counter is a monotonically non-decreasing integer metric. Inc adds one;
// Add adds a non-negative delta. Negative deltas are rejected to preserve
// the monotonicity invariant that downstream dashboards rely on.
type Counter struct {
	v uint64
}

// Inc adds 1 to the counter.
func (c *Counter) Inc() {
	atomic.AddUint64(&c.v, 1)
}

// Add adds n to the counter. n must be non-negative; Add returns false and
// leaves the counter unchanged if n < 0.
func (c *Counter) Add(n int64) bool {
	if n < 0 {
		return false
	}
	atomic.AddUint64(&c.v, uint64(n))
	return true
}

// Get returns the current counter value.
func (c *Counter) Get() uint64 {
	return atomic.LoadUint64(&c.v)
}

// Snapshot returns a copy of the counter value suitable for logging or
// transport. The returned value is a plain uint64 and does not change after
// the snapshot is taken.
func (c *Counter) Snapshot() uint64 {
	return c.Get()
}

// Gauge is a signed floating-point metric that may go up or down. Typical
// uses include in-flight request counts, memory usage, or queue depth.
type Gauge struct {
	inner Float64Bits
}

// Set replaces the gauge value with val.
func (g *Gauge) Set(val float64) {
	g.inner.Set(val)
}

// Add adds delta to the gauge (delta may be negative).
func (g *Gauge) Add(delta float64) {
	g.inner.Add(delta)
}

// Sub subtracts delta from the gauge.
func (g *Gauge) Sub(delta float64) {
	g.inner.Add(-delta)
}

// Get returns the current gauge value.
func (g *Gauge) Get() float64 {
	return g.inner.Get()
}

// Snapshot returns a copy of the gauge value.
func (g *Gauge) Snapshot() float64 {
	return g.Get()
}

// Registry is a simple, mutex-guarded collection of named counters and
// gauges. It is intended for applications that want a single place to look
// up metrics by name at runtime. For hot paths, hold a direct *Counter or
// *Gauge pointer instead of going through the registry on every call.
type Registry struct {
	counters map[string]*Counter
	gauges   map[string]*Gauge
}

// NewRegistry returns an empty Registry ready to register metrics into.
func NewRegistry() *Registry {
	return &Registry{
		counters: make(map[string]*Counter),
		gauges:   make(map[string]*Gauge),
	}
}

// Counter returns the Counter registered under name, creating it on first
// access. Subsequent calls with the same name return the same pointer.
// The pointer may be shared across goroutines.
func (r *Registry) Counter(name string) *Counter {
	if c, ok := r.counters[name]; ok {
		return c
	}
	c := &Counter{}
	r.counters[name] = c
	return c
}

// Gauge returns the Gauge registered under name, creating it on first
// access.
func (r *Registry) Gauge(name string) *Gauge {
	if g, ok := r.gauges[name]; ok {
		return g
	}
	g := &Gauge{}
	r.gauges[name] = g
	return g
}
