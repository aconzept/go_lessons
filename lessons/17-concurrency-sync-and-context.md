# Lesson 17 — Concurrency: `sync`, Worker Pools, `context`

> Back to [roadmap](../roadmap.md).

This lesson covers:
- `sync.WaitGroup` — waiting for goroutines to finish
- `sync.Mutex` and `sync.RWMutex` — protecting shared state
- `sync.Once` — initialise exactly once
- `sync.Map` and the `atomic` package — when (rarely) to use them
- Worker pools, properly wired
- `context` — cancellation, deadlines, request-scoped values

The previous lesson covered the syntax of goroutines and channels. This one is the toolkit you actually use to coordinate them in real programs.

---

## `sync.WaitGroup`

A `WaitGroup` counts goroutines. The orchestrator calls `Add` before launching, each goroutine calls `Done` when finished, and `Wait` blocks until the counter hits zero.

```go
import "sync"

var wg sync.WaitGroup

for _, url := range urls {
    wg.Add(1)
    go func(u string) {
        defer wg.Done()
        fetch(u)
    }(u)
}
wg.Wait()
```

Rules of thumb:

- **`Add` runs on the spawning side, before `go`.** Adding inside the goroutine creates a race where `Wait` might return before the goroutine even starts.
- **`Done` is `defer`red as the first line.** Otherwise a panic skips it and `Wait` hangs forever.
- **A `WaitGroup` is not for one-shot signalling.** Use a channel for that.

`WaitGroup` is also the safe answer to "I close the channel when all senders are done":

```go
results := make(chan Result)
var wg sync.WaitGroup

for _, j := range jobs {
    wg.Add(1)
    go func(j Job) {
        defer wg.Done()
        results <- process(j)
    }(j)
}

go func() {
    wg.Wait()
    close(results)   // single closer — after every sender's Done
}()

for r := range results {
    use(r)
}
```

This is the canonical "wait for workers, then signal end of stream" shape.

### Go 1.25+: `WaitGroup.Go`

In Go 1.25 a convenience method was added that bundles Add+go+defer Done:

```go
var wg sync.WaitGroup

for _, j := range jobs {
    wg.Go(func() { process(j) })
}
wg.Wait()
```

The pattern stays the same, fewer lines. If your `go.mod` says `go 1.25` or later, prefer this form. (On older modules, stick with the explicit `Add`/`Done`.)

---

## `sync.Mutex` and `sync.RWMutex`

A mutex serialises access to shared mutable state. The canonical pattern is to embed it next to the data it guards:

```go
type Counter struct {
    mu sync.Mutex
    n  int
}

func (c *Counter) Inc() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.n++
}

func (c *Counter) Value() int {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.n
}
```

Notes:

- **Always pass `*Counter` around**, never `Counter` by value — copying a struct containing a mutex creates two separate mutexes, which is almost always a bug. `go vet` flags this.
- **`defer Unlock` is the default.** Skip the defer only when the critical section is genuinely tiny and you've measured. Most pretend-optimisations here are wrong.
- **Keep critical sections short.** Don't hold a mutex across I/O, channel sends, or other potentially-blocking calls — that's how you turn a quick lock into a queue-shaped pileup.

### `RWMutex`

Use when reads vastly outnumber writes. Multiple readers can hold the read lock simultaneously; a writer has exclusive access.

```go
var (
    mu    sync.RWMutex
    cache = map[string]string{}
)

func get(k string) (string, bool) {
    mu.RLock()
    defer mu.RUnlock()
    v, ok := cache[k]
    return v, ok
}

func set(k, v string) {
    mu.Lock()
    defer mu.Unlock()
    cache[k] = v
}
```

`RWMutex` has more overhead than a plain `Mutex` and only pays off when the read/write ratio is heavily skewed and contention is real. When in doubt, start with `Mutex` and measure.

### Channels vs mutexes

Both protect against races. Picking between them:

- **Channels** for *handing off ownership* or coordinating *who does what next*.
- **Mutex** for *shared state that's mutated in place* (a cache, a counter, a map of connections).

A common newcomer mistake is using channels for things mutexes do well (a hot counter behind a goroutine becomes a bottleneck). The reverse mistake is using mutexes to send work between goroutines via shared queues, when a channel would express the intent more clearly. Use the simpler one for the job.

---

## `sync.Once`

Runs a function exactly once across any number of callers, no matter how many goroutines race:

```go
var (
    cfgOnce sync.Once
    cfg     *Config
    cfgErr  error
)

func loadConfig() (*Config, error) {
    cfgOnce.Do(func() {
        cfg, cfgErr = parse("/etc/app.yaml")
    })
    return cfg, cfgErr
}
```

This is the idiomatic singleton — lazy and thread-safe. Go 1.21 added `sync.OnceValue[T]` and `sync.OnceValues[T1, T2]` which wrap the same pattern more cleanly:

```go
var loadConfig = sync.OnceValues(func() (*Config, error) {
    return parse("/etc/app.yaml")
})
```

Use the new forms on modern modules.

---

## `sync.Map` and `atomic` — the "rarely" tools

### `sync.Map`

A concurrent map type. Two narrow cases where it actually helps:

1. **Read-mostly, write-rare** maps where the key set is mostly stable.
2. **Sharded by key** — multiple goroutines write to disjoint key spaces.

For everything else, a plain `map` behind a `sync.Mutex` is faster and easier to reason about. The Go docs themselves say so. If you're tempted to use `sync.Map` because it "sounds thread-safe", reach for a mutex instead.

### `sync/atomic`

For lock-free atomic operations on integers and pointers:

```go
import "sync/atomic"

var hits atomic.Int64
hits.Add(1)
n := hits.Load()
```

(`atomic.Int64`, `atomic.Pointer[T]`, etc. were added in Go 1.19; older code uses the free-function form `atomic.AddInt64(&n, 1)`.)

Useful when you have a hot counter or flag and a `Mutex` shows up in your profile. Not a general substitute for mutexes — atomics protect a single word, not invariants across multiple variables.

---

## Worker pools

A worker pool is a fixed group of goroutines reading jobs from a channel. It's the standard answer to "I have N tasks and want to run K of them in parallel without spawning N goroutines."

```go
func runPool(ctx context.Context, jobs []Job, workers int) []Result {
    jobsCh    := make(chan Job)
    resultsCh := make(chan Result)

    var wg sync.WaitGroup
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := range jobsCh {
                select {
                case resultsCh <- process(j):
                case <-ctx.Done():
                    return
                }
            }
        }()
    }

    go func() {
        defer close(jobsCh)
        for _, j := range jobs {
            select {
            case jobsCh <- j:
            case <-ctx.Done():
                return
            }
        }
    }()

    go func() {
        wg.Wait()
        close(resultsCh)
    }()

    var out []Result
    for r := range resultsCh {
        out = append(out, r)
    }
    return out
}
```

What's worth noticing:

- **Both the feeder and each worker check `ctx.Done()`** so cancellation propagates everywhere.
- **One goroutine owns `close(jobsCh)`** (the feeder). One goroutine owns `close(resultsCh)` (the waiter on `wg`). Two separate closers, each responsible for one channel.
- **`workers` is configuration**, not auto-derived. Pick it based on the work: `runtime.NumCPU()` for CPU-bound work, much higher for I/O-bound work, often `1` for cases where ordering matters.

For most real code, the `errgroup` package (`golang.org/x/sync/errgroup`) is a nicer ergonomic over this pattern — it handles the WaitGroup, the cancellation, and the first-error semantics in one type. Worth knowing it exists; we'll touch it in passing in patterns.

---

## `context`

`context.Context` is the standard way to carry **cancellation signals, deadlines, and request-scoped values** through a call tree.

Every function that does I/O, blocking work, or spawns goroutines should accept a `context.Context` as its first argument:

```go
func Fetch(ctx context.Context, url string) ([]byte, error) { ... }
```

This is so universal in Go codebases that "no `ctx`" is itself a code smell on those signatures.

### Cancellation

A context is a tree. Cancelling a parent cancels every descendant. The two everyday constructors:

```go
ctx, cancel := context.WithCancel(parent)
defer cancel()                    // ALWAYS defer cancel — releases resources

ctx, cancel := context.WithTimeout(parent, 2*time.Second)
defer cancel()

ctx, cancel := context.WithDeadline(parent, time.Now().Add(5*time.Second))
defer cancel()
```

Receivers cooperate by selecting on `ctx.Done()`:

```go
select {
case v := <-ch:
    use(v)
case <-ctx.Done():
    return ctx.Err()        // either context.Canceled or context.DeadlineExceeded
}
```

`ctx.Err()` returns `nil` until the context is cancelled, then the reason. `ctx.Done()` returns a channel that closes on cancellation — so it composes cleanly in `select`.

### The two "no real parent" contexts

```go
context.Background()   // top of a server / main()
context.TODO()         // "I'll plumb a real context through later"
```

Use `Background` at process roots. Use `TODO` as a literal note to your future self that this site doesn't have a real context yet — `go vet`/linters can flag remaining ones.

### Common use cases

1. **HTTP request scope.** `r.Context()` on an `*http.Request` is cancelled when the client disconnects. Propagate it into downstream calls (DB, RPC, fanned-out goroutines).
2. **Bounded operations.** `WithTimeout`/`WithDeadline` to enforce "this call must finish in X". The DB driver, the HTTP client, the gRPC stub — all of them respect contexts and cancel their network ops.
3. **Goroutine lifecycle.** Pass `ctx` into every worker; bail when `<-ctx.Done()`.

### Values — sparingly

```go
ctx := context.WithValue(parent, traceIDKey, "abc-123")
v := ctx.Value(traceIDKey).(string)
```

This works but is widely misused. **Use it for cross-cutting request metadata** — request IDs, auth principals, trace spans. Don't use it as a backdoor for passing arguments your function should take explicitly.

Two strong conventions:
- The key must be an unexported, named type so different packages can't collide: `type traceIDKey struct{}`.
- Wrap with a typed accessor: `func TraceID(ctx context.Context) (string, bool)`.

### Things `context` is *not*

- Not a way to *kill* a goroutine that's wedged in CPU-bound code with no `select`. The goroutine has to *choose* to check the context.
- Not a substitute for application state.
- Not optional. Add it to your signatures even before you need it; retrofitting `context` later through a deep call tree is painful.

---

## Recap

- `WaitGroup` to wait for goroutines; `Add` outside, `defer Done` inside; on Go 1.25+ use `wg.Go`.
- `Mutex` for shared mutable state; copy-of-mutex is a bug. `RWMutex` only when reads massively dominate.
- `sync.Once` / `OnceValue` for one-shot lazy init.
- `sync.Map` and `atomic` exist; they aren't your defaults.
- Worker pools are channel + N goroutines + WaitGroup + context cancellation. `errgroup` packages this nicely.
- `context.Context` is the universal cancellation/deadline carrier. Pass it as the first argument, defer the `cancel`, select on `Done()`.

Next: **Concurrency Part 3 — patterns (fan-in, fan-out, pipeline) and the race detector**.

[← Back to roadmap](../roadmap.md)
