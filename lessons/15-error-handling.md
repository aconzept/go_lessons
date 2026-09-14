# Lesson 15 — Error Handling

> Back to [roadmap](../roadmap.md).

This lesson covers:
- The `error` interface
- The "return-and-check" pattern
- `errors.New` and `fmt.Errorf`
- Wrapping with `%w`, `errors.Is`, `errors.As`
- Sentinel errors and custom error types
- `panic` / `recover` — what they're really for
- Stack traces and debugging

Go does not have exceptions. Errors are **values** of type `error`, returned alongside results, and checked explicitly. The trade-off: more typing and ceremony in source, but every failure path is visible in the diff.

---

## The `error` interface

There's no magic — `error` is a single-method interface in the standard library:

```go
type error interface {
    Error() string
}
```

Any type with an `Error() string` method satisfies it. The zero value of an interface is `nil`, so "no error" is just `nil`.

The canonical signature:

```go
func DoThing() (Result, error)
```

When success: return `result, nil`. When failure: return the zero `Result`, plus a non-nil error. Callers handle it immediately:

```go
r, err := DoThing()
if err != nil {
    return err            // or wrap, log, recover...
}
use(r)
```

That `if err != nil { return err }` will be the most-typed line in your Go career.

---

## Creating errors

### `errors.New`

For a fixed string with no formatting:

```go
import "errors"

var ErrNotFound = errors.New("not found")
```

### `fmt.Errorf`

For a formatted message:

```go
import "fmt"

return fmt.Errorf("read config %q: file too large (%d bytes)", path, size)
```

`fmt.Errorf` uses the same verbs as `fmt.Sprintf` — `%s`, `%d`, `%v`, etc. One verb is special: `%w` wraps an underlying error.

---

## Wrapping with `%w`

Wrapping preserves the *chain* of errors. The outer error adds context; the original error is still inspectable underneath.

```go
if _, err := os.Open(path); err != nil {
    return fmt.Errorf("load config: %w", err)
}
```

The caller sees a message like `load config: open /etc/app.yaml: no such file or directory`, but can still ask "was this a 'not exist' error?":

```go
import "errors"

if errors.Is(err, os.ErrNotExist) {
    // handle the missing-file case specifically
}
```

`errors.Is(err, target)` walks the wrap chain and returns true if any link **is** the target (compared by `==` for sentinel errors, or via a custom `Is` method for typed errors).

`errors.As(err, &target)` does the same walk but assigns the first matching link into `target` — useful when the underlying error is a struct you want to inspect:

```go
var perr *fs.PathError
if errors.As(err, &perr) {
    log.Printf("path that failed: %s", perr.Path)
}
```

**Rule of thumb:** use `%w` when adding context to an error you're propagating. Use `%s` (or `%v`) when you genuinely want to flatten — typically when logging at the boundary and discarding the original.

Only wrap **once** at each layer that adds meaningful context. A six-deep chain of `failed to: failed to: failed to:` is a sign of over-wrapping.

---

## Sentinel errors

A sentinel is a named, exported error value:

```go
var ErrNotFound = errors.New("user not found")

func Find(id int) (*User, error) {
    if id == 0 {
        return nil, ErrNotFound
    }
    // ...
}

// caller:
u, err := Find(42)
if errors.Is(err, ErrNotFound) { ... }
```

Standard library examples: `io.EOF`, `sql.ErrNoRows`, `os.ErrNotExist`, `context.Canceled`, `context.DeadlineExceeded`. Compare against them with `errors.Is`, not `==` — the actual error may be wrapped.

When to use:

- The condition is a stable, well-known *kind* with no extra data.
- Callers will branch on it (`if errors.Is(err, ErrNoRows) { ... }`).

Don't make a sentinel for every error in your package. Most errors are descriptive strings nobody branches on; turning them all into sentinels just adds API surface.

---

## Custom error types

When the error carries data you want callers to read:

```go
type ValidationError struct {
    Field string
    Rule  string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("%s violates %s", e.Field, e.Rule)
}

// Usage
return &ValidationError{Field: "email", Rule: "format"}

// Caller
var verr *ValidationError
if errors.As(err, &verr) {
    return fmt.Sprintf("bad %s", verr.Field), 400
}
```

Conventions:
- The type is usually `XxxError`.
- The pointer form (`*ValidationError`) is the one with the method — that means a `*ValidationError` is the `error`, not a value.
- `errors.As` finds it through any wrapper layers.

If you want `errors.Is(err, target)` to do something other than identity, implement `Is(target error) bool` on the type — but you rarely need to; `errors.As` is the right tool for typed errors.

---

## The "wrap, don't catch" mental model

Pseudocode of a typical service call stack:

```go
func handle(w http.ResponseWriter, r *http.Request) {
    if err := saveOrder(r.Context(), order); err != nil {
        // top level: log + classify + respond
        if errors.Is(err, ErrOutOfStock) {
            http.Error(w, "out of stock", 409); return
        }
        log.Error("save order", "err", err)
        http.Error(w, "internal error", 500)
    }
}

func saveOrder(ctx context.Context, o Order) error {
    if err := stock.Reserve(ctx, o); err != nil {
        return fmt.Errorf("reserve stock: %w", err)   // add layer of context
    }
    if err := db.Insert(ctx, o); err != nil {
        return fmt.Errorf("insert order %d: %w", o.ID, err)
    }
    return nil
}
```

The pattern: **wrap at each layer that has more context to add; check at the layer that has to decide what to do**. Middle layers don't `log.Printf` and they don't `os.Exit`. They return.

---

## `panic` and `recover`

`panic` aborts the current goroutine, unwinds the stack running any deferred functions, and crashes the program if nothing catches it. `recover` (only useful inside a deferred function) stops the unwinding and returns the panic value.

```go
func safeCall() (err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("panic: %v", r)
        }
    }()
    riskyThing()
    return nil
}
```

**Panics are not for control flow.** They exist for two things:

1. **Truly unrecoverable bugs.** "This map lookup must succeed because we just inserted it." A panic here surfaces a programming error loudly, with a stack trace.
2. **Recovering at process boundaries** — the top of a goroutine, the top of an HTTP handler — so a single bad request can't crash the whole server. The Go HTTP server already does this for you per request.

Things to know:

- The standard library panics for some genuine misuses (`json.Marshal` does not, but `sort.Sort` does on nil slices in some cases; indexing out of bounds always panics).
- A panic in one goroutine **does not** propagate to others. It only crashes the whole process if it reaches the top of *its* goroutine. So every goroutine that does risky work should have its own `recover` at the top, especially in libraries.
- After `recover`, you're holding a `any`. Type-assert if you want the original type, and prefer wrapping it in an `error` to propagate further.

Don't use panic/recover to skip writing `if err != nil`. That's the C++/Java instinct; it loses information and surprises the next reader.

---

## Stack traces

When a panic isn't caught, the runtime prints the message and a full stack trace to stderr and exits with code 2. That's free for crashes.

For *errors* — the values you return — Go does **not** automatically capture a stack trace. By the time the caller reads it, the call site is gone. If you want one, options:

- `runtime/debug.Stack()` returns the current stack as a `[]byte`. Capture it when constructing an error if you need it.
- The third-party package `github.com/pkg/errors` (and its successor patterns) builds stack-capturing errors. The 1.13+ `%w` chain doesn't carry stacks; some teams still use the third-party form when they want them.

In practice, most server-side code logs the *wrapped chain* (`fmt.Errorf("...: %w", err)`) at the top of the request and that turns out to be enough — the wrap message names the layer, which is what you actually want to know.

The `go test` tooling prints stack traces automatically on `t.Fatal` and on uncaught panics in tests.

---

## A short checklist

- Every error returned in your code should either:
  - **Add context** with `fmt.Errorf("doing X: %w", err)`, or
  - **Be the original** (you're at the layer where the error was born).
- Compare with `errors.Is` / `errors.As`, never with `==` and never with `strings.Contains(err.Error(), ...)`.
- Don't log *and* return — that's two reports for one problem. Pick one (usually: return, log at the boundary).
- Don't `panic` for expected failures. Don't `recover` deep inside business logic.

---

## Recap

- `error` is a one-method interface; "no error" is `nil`. The canonical signature returns `(T, error)`.
- Build errors with `errors.New` or `fmt.Errorf`. Wrap with `%w` to keep the chain.
- Inspect chains with `errors.Is` (sentinel/equality) and `errors.As` (typed). Don't string-match.
- Sentinel errors for stable "kinds"; custom error types when callers need data.
- `panic`/`recover` are for unrecoverable bugs and process-boundary safety nets, not flow control.

Next: **Concurrency Part 1 — Goroutines & Channels**.

[← Back to roadmap](../roadmap.md)
