# mini_metric_counter

Thread-safe counters and gauges for recording application metrics with zero external dependencies.

```go
package main

import minimetriccounter

func main() {
	reg := minimetriccounter.NewRegistry()

	requests := reg.Counter("http_requests_total")
	requests.Inc()
	requests.Add(5)

	inflight := reg.Gauge("in_flight")
	inflight.Set(10.0)
	inflight.Add(1.0)
	inflight.Sub(2.0)

	_ = requests.Snapshot() // uint64
	_ = inflight.Snapshot() // float64
}
```

## Why this exists

Most Go applications need a handful of simple metrics — request counts,
in-flight gauges, queue depth — without pulling in a full observability
stack. This library provides exactly those two primitives (`Counter` and
`Gauge`) plus a name-based `Registry`, using only `sync/atomic` and
`math` from the standard library. The trade-off: there is no labels,
histograms, or exposition format. If you need those, use a real metrics
library. This is for the case where a counter and a gauge are enough.

`Counter` values are `uint64` and reject negative deltas to stay
monotonic. `Gauge` values are `float64` and may go negative. Because
`sync/atomic` does not expose atomic float64 operations directly, gauges
convert through `math.Float64bits` and CAS-loop on writes — the standard
workaround documented by the Go team.

## Edge cases

- `Counter.Add(n)` with `n < 0` returns `false` and does nothing. Callers
  that need to reset should create a new counter rather than decrement.
- The `Registry` is not safe to register new metrics into while other
goroutines are concurrently registering metrics with the same name. The
returned `*Counter` / `*Gauge` pointers themselves are safe to use from
any goroutine once obtained.
- Float comparisons in tests use a tolerance (`math.Abs(got-want) < eps`);
never compare gauge values with `==`.
