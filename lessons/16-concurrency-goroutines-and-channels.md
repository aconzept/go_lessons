# Lesson 16 — Concurrency: Goroutines & Channels

> Back to [roadmap](../roadmap.md).

This lesson covers:
- Goroutines and the `go` keyword
- Channels: send, receive, close
- Buffered vs unbuffered
- `select`
- The producer/consumer pattern
- The two ground rules of safe concurrency

Concurrency is what Go is famous for. The model: **goroutines** are cheap concurrent functions, **channels** are typed pipes between them. You don't write callbacks, you don't `await`, and you don't think in terms of threads. The runtime multiplexes thousands or millions of goroutines onto a small pool of OS threads.

If you're coming from JS/TS: goroutines are not `async` functions and channels are not promises. Closer cousins: Erlang processes, or Communicating Sequential Processes (CSP) — Go's design is explicitly based on Tony Hoare's CSP paper. Crucially, **goroutines run in parallel on multiple cores**, not on a single event loop.

The next lesson covers `sync`, `context`, patterns (fan-in/out/pipeline), and the race detector. Start here.

---

## Goroutines

Prefix any function call with `go`:

```go
go work()

go func() {
    time.Sleep(100 * time.Millisecond)
    fmt.Println("done")
}()
```

That's it. The `go` statement launches the call as a new goroutine and returns immediately — `go` doesn't wait, doesn't yield, doesn't return anything. The calling code keeps going.

Properties:

- **Cheap.** Each starts with ~2 KB of stack (vs ~1 MB for an OS thread), grown and shrunk by the runtime as needed. Spawning 100,000 goroutines is normal; spawning a million is doable.
- **Scheduled by the Go runtime**, not the OS, onto a pool of OS threads (controlled by `GOMAXPROCS`, default = CPU count).
- **No return value.** If you want a result, send it through a channel.
- **No exception propagation.** A panic in one goroutine doesn't surface in another — it crashes the program if not recovered locally. Top-level recover is a libraries-and-servers concern.

### `main` doesn't wait

When `main` returns, the program exits — *running goroutines are killed mid-flight*. So this prints nothing:

```go
func main() {
    go fmt.Println("hello")
    // main returns immediately
}
```

Coordinate explicitly with a channel, a `sync.WaitGroup`, or a `context`. Don't sleep — it papers over the bug instead of fixing it.

---

## Channels

A channel is a typed conduit:

```go
ch := make(chan int)       // unbuffered, of int
ch := make(chan string, 5) // buffered, capacity 5
```

Operations:

```go
ch <- 42        // send (blocks if the channel can't accept right now)
v := <-ch       // receive (blocks if there's nothing to receive)
v, ok := <-ch   // receive; ok=false if the channel was closed and drained
close(ch)       // close (only the sender should do this)
```

Channels are passed by *value*, but the value is a small handle — the channel itself isn't copied. Two goroutines holding the same `ch` see the same channel.

### Unbuffered channels — synchronisation, not just data

```go
ch := make(chan int)
```

`ch <- v` blocks until *some other* goroutine is ready to `<-ch`, and vice versa. The send and the receive happen together. Unbuffered channels are how you do **rendezvous** synchronisation:

```go
done := make(chan struct{})

go func() {
    work()
    done <- struct{}{}     // signal "work finished"
}()

<-done                     // wait for it
```

`chan struct{}` carries no data — it's a pure signal. `struct{}` is the zero-byte type.

### Buffered channels — queue with backpressure

```go
ch := make(chan int, 3)
ch <- 1
ch <- 2
ch <- 3
ch <- 4   // BLOCKS — buffer is full, until someone receives
```

A buffered channel decouples sender and receiver up to `cap` items. Sending blocks when the buffer is full; receiving blocks when it's empty.

**Pick a size based on the design**, not for performance. Common sizes:

- `0` (unbuffered) — for "hand off one thing", rendezvous.
- `1` — for "latest value wins" patterns, or to send a signal without blocking when the receiver might be slightly late.
- `N` matching the number of workers — for a fan-out queue.
- Larger — when you genuinely need a queue and accept that consumers can fall behind.

Buffering does **not** mean "send won't block ever". It means "send won't block until the buffer is full". If your goal is "never block", you want `select` with a `default` branch (below).

### Direction-typed channels

Function parameters can constrain direction:

```go
func produce(out chan<- int) { out <- 42 }     // send-only
func consume(in <-chan int)  { fmt.Println(<-in) } // receive-only
```

A bidirectional `chan int` converts implicitly to either restricted form. Use direction types on parameters as documentation and protection — the compiler will then reject misuse.

### Close, range, and the closed-channel rules

```go
close(ch)            // mark closed; cannot send anymore (panics if you do)
v, ok := <-ch        // when closed and drained, ok=false and v is zero
for v := range ch { ... }   // iterates until ch is closed and drained
```

Rules of thumb:

- **Only the sender closes.** If multiple goroutines send, they have to coordinate — usually via a `sync.WaitGroup` and a single closer.
- **Close to signal "no more values", not to free resources.** A closed channel isn't garbage-collected sooner.
- **Receiving from a closed channel never blocks** — it returns the zero value immediately, repeatedly. The `, ok` form is how you detect closure.
- **Sending on a closed channel panics.** This is one of the rare runtime panics that is genuinely fatal in normal code.

Half the channel bugs newcomers write involve closing too early or from too many places. The simple discipline: one sender goroutine per channel; that one closes when done.

---

## `select`

`select` is `switch` for channel operations. Each case is a send or a receive; the first one ready runs. If none are ready, `select` blocks. With a `default` branch, it never blocks.

```go
select {
case v := <-incoming:
    handle(v)
case out <- nextItem:
    advance()
case <-time.After(time.Second):
    // timeout
case <-ctx.Done():
    return ctx.Err()
}
```

`select` is what makes channels composable. Common shapes:

### Timeout

```go
select {
case v := <-ch:
    use(v)
case <-time.After(2 * time.Second):
    return errors.New("timeout")
}
```

`time.After` allocates a new timer each call — fine for one-shots, wasteful in tight loops. For loops, build a `time.NewTimer` once and `Reset` it, or use `context.WithTimeout`.

### Non-blocking send/receive

```go
// try-send
select {
case ch <- v:
    // sent
default:
    // would block; drop or buffer
}

// try-receive
select {
case v := <-ch:
    use(v)
default:
    // nothing there
}
```

The `default` branch fires when no case is ready. Useful for "best effort" patterns like metrics emission where you'd rather drop than block.

### Cancellation

```go
for {
    select {
    case job := <-jobs:
        process(job)
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

This shape is everywhere — a long-running goroutine that processes channel input *or* exits when its context is cancelled. We'll formalise `context` in the next lesson.

---

## Producer / consumer — putting it together

The minimal "worker reads from a channel" pattern:

```go
func worker(jobs <-chan Job, results chan<- Result) {
    for j := range jobs {
        results <- process(j)
    }
}

func main() {
    jobs    := make(chan Job, 100)
    results := make(chan Result, 100)

    // start three workers
    for i := 0; i < 3; i++ {
        go worker(jobs, results)
    }

    // feed jobs
    go func() {
        for _, j := range loadJobs() {
            jobs <- j
        }
        close(jobs)   // tells workers "no more"
    }()

    // collect — but we need to know how many to expect
    for i := 0; i < total; i++ {
        fmt.Println(<-results)
    }
}
```

Notice the awkward "we need to know how many to expect" comment — that's what `sync.WaitGroup` solves (next lesson). For now: the pattern is real, the gap is what we close next.

---

## The two ground rules of safe concurrency

After all the syntax, two principles:

### 1. "Don't communicate by sharing memory; share memory by communicating."

Famous Pike line. Translation: prefer to *pass data through channels* between goroutines, rather than have multiple goroutines mutate a shared variable. Channels eliminate a class of data races because the design ensures only one goroutine "owns" the data at a time.

When you do need shared mutable state — counters, maps, caches — that's what `sync.Mutex` and `sync.RWMutex` are for. Pick the channel-shaped solution when it fits; reach for mutexes when it doesn't.

### 2. The race detector finds the rest.

```
go test -race ./...
go run -race .
go build -race
```

This instruments your code with runtime checks for data races. It's not exhaustive (it only catches races on code paths your tests actually run), but it's astonishingly good at flagging real bugs the moment they happen.

Run `-race` in CI for any package that uses concurrency. The slowdown is usually 2–10×; in tests that's fine.

---

## Recap

- `go f()` launches a goroutine. They're cheap, scheduled by the runtime, and panic in isolation.
- Channels are typed pipes — unbuffered for rendezvous, buffered for backpressure. Only the sender closes.
- `select` makes channels composable; `default` makes it non-blocking; pair with `ctx.Done()` for cancellation.
- "Share memory by communicating." Mutexes are the fallback, not the default.
- Always run `-race` on concurrent code.

Next: **Concurrency Part 2 — `sync`, `context`, patterns, and the race detector**.

[← Back to roadmap](../roadmap.md)
