# Lesson 18 — Concurrency: Patterns & Race Detection

> Back to [roadmap](../roadmap.md).

This lesson covers:
- Fan-out — distributing work across goroutines
- Fan-in — merging multiple channels into one
- Pipelines — composable stages of stream processing
- A pragmatic `errgroup` mention
- The race detector — what it catches and how to use it
- A short field guide to common races

Last lesson covered the primitives. This one builds the three patterns you'll reuse, then turns to the safety net every concurrent codebase needs: `-race`.

---

## Fan-out

**Fan-out** = one producer hands work to N consumers running in parallel. It's the worker pool from the previous lesson — included here under its standard name.

```go
func fanOut(ctx context.Context, jobs <-chan Job, n int) <-chan Result {
    out := make(chan Result)
    var wg sync.WaitGroup

    for i := 0; i < n; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := range jobs {
                select {
                case out <- process(j):
                case <-ctx.Done():
                    return
                }
            }
        }()
    }

    go func() {
        wg.Wait()
        close(out)
    }()

    return out
}
```

Pick `n`:

- **CPU-bound work** — `runtime.NumCPU()`, possibly minus one if the orchestrator also does work.
- **I/O-bound work** — much higher, often hundreds. The goroutine is cheap; the open file descriptor or HTTP connection is the real limit.
- **Ordering matters** — `n = 1`. Don't fan out at all.

---

## Fan-in

**Fan-in** = N producers feed into one consumer through a merged channel.

```go
func fanIn[T any](ctx context.Context, channels ...<-chan T) <-chan T {
    out := make(chan T)
    var wg sync.WaitGroup

    wg.Add(len(channels))
    for _, c := range channels {
        c := c     // pre-1.22 capture safety; harmless on newer Go
        go func() {
            defer wg.Done()
            for v := range c {
                select {
                case out <- v:
                case <-ctx.Done():
                    return
                }
            }
        }()
    }

    go func() {
        wg.Wait()
        close(out)
    }()

    return out
}
```

The shape is the mirror of fan-out: each source has a goroutine forwarding into the shared sink; one goroutine waits and closes the sink.

This is the pattern behind things like merging multiple watchers, multiplexing event streams, or collecting partial results from sharded queries.

---

## Pipelines

A **pipeline** is a chain of stages connected by channels. Each stage:

1. Receives values on an input channel.
2. Does its work.
3. Sends results on an output channel.
4. Closes the output when its input is closed.

```go
func gen(ctx context.Context, nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for _, n := range nums {
            select {
            case out <- n:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

func square(ctx context.Context, in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for n := range in {
            select {
            case out <- n * n:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

func sum(in <-chan int) int {
    total := 0
    for n := range in {
        total += n
    }
    return total
}

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    nums    := gen(ctx, 1, 2, 3, 4, 5)
    squares := square(ctx, nums)
    fmt.Println(sum(squares))   // 1 + 4 + 9 + 16 + 25 = 55
}
```

Three principles that hold every pipeline together:

1. **Stages own their output channel** and close it on exit. Downstream stages stop their `range` cleanly.
2. **Every stage checks `ctx.Done()`** so cancellation drains the pipeline from the top.
3. **Backpressure is automatic.** If `sum` is slow, `square` blocks on send, `gen` blocks on send, no buffer fills up.

Combine pipelines with fan-out/fan-in to parallelise heavy stages:

```
gen → square (×8 fan-out → fan-in) → sum
```

The "expensive middle stage" is the place to throw goroutines at.

### A leak pattern to know

If a downstream consumer **stops reading before the producer is done**, the producer blocks forever — a *goroutine leak*. The fix is what we did above: every stage selects on `ctx.Done()`, and the caller cancels the context when it's done early.

A surprising number of "my server slowly runs out of memory" bugs are leaked pipeline goroutines. The race detector won't catch this (no shared-memory race). The goroutine profile will (`runtime/pprof`, the `goroutine` profile — covered later in the Performance & Debugging section).

---

## A pragmatic note on `errgroup`

The patterns above are educational and you should be able to read them. In real code, reach for `golang.org/x/sync/errgroup` for any "fan out, collect first error, cancel on failure" job:

```go
import "golang.org/x/sync/errgroup"

func process(ctx context.Context, urls []string) error {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(8)                       // cap on concurrent goroutines

    for _, u := range urls {
        g.Go(func() error {
            return fetch(ctx, u)        // any error → ctx is cancelled, others bail
        })
    }
    return g.Wait()                     // returns the first non-nil error
}
```

That's a fan-out plus a worker-pool cap plus error propagation, in ~10 lines. It's the standard answer when "do these N things in parallel, fail fast" describes the job.

---

## The race detector

A **data race** is two goroutines accessing the same memory location concurrently where at least one is writing, with no synchronisation between them. In Go this is **always a bug** — the memory model only gives guarantees in the presence of explicit synchronisation (channel ops, mutex ops, atomics, `sync.Once`, etc.).

A race is not just "wrong output" — it's undefined behaviour. A racy program can corrupt memory, infinite-loop the runtime, or produce values that violate the language's own invariants. There is no acceptable race in Go.

### Running with `-race`

```bash
go test  -race ./...
go run   -race ./cmd/server
go build -race -o bin/server ./cmd/server
```

The `-race` flag enables runtime instrumentation. When two unsynchronised accesses to the same address are detected at runtime, you get an output like:

```
==================
WARNING: DATA RACE
Read at 0x00c00009c008 by goroutine 7:
  main.observe()
      /home/me/proj/main.go:21 +0x44

Previous write at 0x00c00009c008 by goroutine 6:
  main.increment()
      /home/me/proj/main.go:14 +0x52

Goroutine 7 (running) created at:
  main.main()
      /home/me/proj/main.go:34 +0xc8
...
==================
```

Two stack traces — the read and the write that race — plus the goroutines they happen on. That's usually enough to fix it.

### What it catches, and what it doesn't

**Catches:**

- Reads/writes to shared variables (ints, slices, maps, structs) without synchronisation.
- Races involving channels you might not expect (e.g. closing a channel concurrently with a send).
- Races in code paths your tests actually execute.

**Does not catch:**

- Deadlocks. (The runtime separately detects all-goroutines-blocked deadlocks; partial deadlocks need other tooling.)
- Goroutine leaks. (Use `goleak` in tests, or watch the `goroutine` profile.)
- Logical concurrency bugs that don't involve a memory race — e.g. processing the same job twice.
- Races on code paths your tests don't run.

The last one is why "we run `-race` in CI" is necessary but not sufficient. Coverage of your concurrent code is what makes `-race` useful.

### Cost

`-race` instrumentation typically slows code 5–15× and increases memory ~5–10×. Fine for tests; sometimes fine for staging; almost never fine for production. The recommended posture:

- **CI**: `go test -race ./...` on every PR.
- **Local**: enable when developing concurrent code; disable when running benchmarks.
- **Production**: off, unless you're chasing a specific bug you can't reproduce.

### A short field guide

Patterns that the race detector reliably flags:

```go
// 1. Closure capture without protection
var sum int
for _, x := range nums {
    go func() { sum += x }()       // race on sum (and on x pre-1.22)
}

// 2. Unprotected map writes
m := map[string]int{}
for _, k := range keys {
    go func(k string) { m[k]++ }(k)   // race
}

// 3. Reading a value being mutated
go writer(&cfg)
go reader(&cfg)                       // race if cfg has any non-atomic field

// 4. WaitGroup misuse
var wg sync.WaitGroup
go func() {
    wg.Add(1)                         // race with Wait — Add before launching
    defer wg.Done()
    work()
}()
wg.Wait()
```

For each, the fix is one of: mutex, channel hand-off, atomic, or restructuring so only one goroutine writes.

---

## Recap

- **Fan-out**: 1 → N workers; pick N from the work type.
- **Fan-in**: N → 1 merged channel; mirror of fan-out.
- **Pipeline**: stages connected by channels; each stage owns and closes its output; every stage respects `ctx.Done()` to avoid leaks.
- Use `errgroup` for "fan out and fail fast" — it's the practical default.
- Run `go test -race ./...` in CI. Races are always bugs. The detector catches what your tests cover; goroutine leaks and deadlocks need separate tools.

This wraps the Concurrency section of the roadmap. Next would be **Testing & Benchmarking**.

[← Back to roadmap](../roadmap.md)
