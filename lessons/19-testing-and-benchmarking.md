# Lesson 19 — Testing & Benchmarking

> Back to [roadmap](../roadmap.md).

This lesson covers:
- The `testing` package and how `go test` finds tests
- Table-driven tests and subtests
- Helpers, parallelism, and cleanup
- Mocks, stubs, and fakes — the Go idiom
- `httptest` for HTTP testing
- Benchmarks and what `b.N` means
- Coverage and fuzz testing
- Example tests as runnable docs

Testing is built into the toolchain. There's no Jest, no Mocha, no install step. Tests live next to the code, are written in the same language, and run with `go test`.

---

## The basics

A test is a function named `TestXxx` in a `*_test.go` file, taking a single `*testing.T`:

```go
// math_test.go
package math

import "testing"

func TestAdd(t *testing.T) {
    got := Add(2, 3)
    if got != 5 {
        t.Errorf("Add(2,3) = %d; want 5", got)
    }
}
```

Run it:

```bash
go test ./...                 # everything
go test ./math                # one package
go test -run TestAdd -v ./math   # one test, verbose
```

A few things to know upfront:

- **`*_test.go` files are excluded from normal builds.** Only `go test` compiles them. So test code never bloats your binary.
- **Test files live next to the code** they test, in the same package. To test from outside (black-box style), put them in `package foo_test` instead of `package foo` — same directory, different package.
- **Two failure verbs:** `t.Errorf` records the failure but the test keeps running; `t.Fatalf` records and stops the test immediately. Use `Fatalf` when the next line would crash or be meaningless if the assertion is wrong.
- There's no `assert.Equal`. The community standard is "print what you got, what you wanted, and let `go test` show the diff." Many shops add `github.com/google/go-cmp/cmp` for structured diffs of complex types.

### Setup, teardown, helpers

```go
func TestSomething(t *testing.T) {
    db := newTestDB(t)
    t.Cleanup(func() { db.Close() })

    // ... rest of test
}
```

- `t.Cleanup(fn)` registers a function to run when the test ends. Use it instead of `defer` when the resource is set up in a helper that returns to the test.
- `t.Helper()` inside a helper function makes the failure line point at the caller, not at the helper:
  ```go
  func mustParse(t *testing.T, s string) int {
      t.Helper()
      n, err := strconv.Atoi(s)
      if err != nil { t.Fatalf("parse %q: %v", s, err) }
      return n
  }
  ```

There is no global `BeforeAll`/`AfterEach`. Each test sets up what it needs. Tests that share expensive setup use a `TestMain`:

```go
func TestMain(m *testing.M) {
    // setup
    code := m.Run()
    // teardown
    os.Exit(code)
}
```

Use `TestMain` sparingly — most tests do better with per-test helpers.

---

## Table-driven tests

The dominant Go testing style. Define a slice of cases, loop with `t.Run` to give each one a name:

```go
func TestClamp(t *testing.T) {
    cases := []struct {
        name        string
        v, lo, hi   int
        want        int
    }{
        {"below",   -1, 0, 10,  0},
        {"above",   99, 0, 10, 10},
        {"inside",   5, 0, 10,  5},
        {"at low",   0, 0, 10,  0},
        {"at high", 10, 0, 10, 10},
    }

    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            got := Clamp(c.v, c.lo, c.hi)
            if got != c.want {
                t.Errorf("Clamp(%d,%d,%d) = %d; want %d",
                    c.v, c.lo, c.hi, got, c.want)
            }
        })
    }
}
```

Why this scales:

- **Subtests have names**, so failures point at exactly the case that broke.
- **You can run one case** with `go test -run TestClamp/below`.
- **Cases live in one place** — adding a case is one struct literal.
- **`t.Parallel()` inside the subtest** lets cases run concurrently when they're independent.

When the table grows past ~15 rows or the inputs are big, extract the cases to a fixture file or split into multiple tests. The table is a tool, not a religion.

---

## Parallelism

Tests are sequential by default; subtests inside a `Run` can opt into parallel execution:

```go
for _, c := range cases {
    c := c   // pre-1.22 capture safety
    t.Run(c.name, func(t *testing.T) {
        t.Parallel()
        // ...
    })
}
```

`-parallel N` controls the cap (default `GOMAXPROCS`). The first level (`TestXxx` itself) goes parallel with `t.Parallel()` at the top of the test body — but only against other tests that also called `Parallel`.

If a test reads or writes shared state, **don't** make it parallel, or guard the state with a mutex. The race detector will help you find these.

---

## Mocks, stubs, and fakes

Go has no built-in mocking framework. The community's overwhelming preference: **define small interfaces at the consumer side, and substitute hand-written fakes in tests**.

```go
// Production code
type UserStore interface {
    Find(ctx context.Context, id int) (*User, error)
}

type Service struct {
    users UserStore
}

func (s *Service) Greet(ctx context.Context, id int) (string, error) {
    u, err := s.users.Find(ctx, id)
    if err != nil {
        return "", err
    }
    return "hi, " + u.Name, nil
}

// Test code
type fakeUsers struct{ byID map[int]*User }

func (f *fakeUsers) Find(_ context.Context, id int) (*User, error) {
    u, ok := f.byID[id]
    if !ok { return nil, errors.New("not found") }
    return u, nil
}

func TestGreet(t *testing.T) {
    s := Service{users: &fakeUsers{byID: map[int]*User{1: {Name: "Ada"}}}}
    got, err := s.Greet(context.Background(), 1)
    if err != nil || got != "hi, Ada" {
        t.Fatalf("Greet(1) = (%q, %v)", got, err)
    }
}
```

Notes:

- **The interface lives next to `Service`**, not next to `UserStore`'s implementation. That's the "interfaces at the consumer" rule from the interfaces lesson — it pays off here.
- **A 20-line fake is usually clearer than a mock framework.** When call-order verification really matters, `gomock` or `mockery` generate this for you, but reach for them only when the hand-written version would be tedious.
- **Stubs**: returns canned answers. **Fakes**: a working in-memory implementation. **Mocks**: record calls and assert on them. Most Go tests need fakes, occasionally stubs, rarely mocks.

---

## `httptest` for HTTP

`net/http/httptest` lets you spin up a real HTTP server (against a free local port) or a fake `ResponseWriter`. Two patterns cover most needs.

### Testing a handler with `httptest.NewRecorder`

```go
func TestHelloHandler(t *testing.T) {
    req := httptest.NewRequest(http.MethodGet, "/hello?name=Ada", nil)
    rec := httptest.NewRecorder()

    helloHandler(rec, req)

    res := rec.Result()
    defer res.Body.Close()

    if res.StatusCode != http.StatusOK {
        t.Fatalf("status = %d", res.StatusCode)
    }
    body, _ := io.ReadAll(res.Body)
    if string(body) != "hi, Ada" {
        t.Errorf("body = %q", body)
    }
}
```

No port, no networking. Fast and deterministic — what you want for handler unit tests.

### Faking a downstream server with `httptest.NewServer`

```go
func TestFetchUser(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/users/42" {
            http.NotFound(w, r); return
        }
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprint(w, `{"id":42,"name":"Ada"}`)
    }))
    defer srv.Close()

    u, err := FetchUser(srv.URL, 42)
    if err != nil { t.Fatal(err) }
    if u.Name != "Ada" {
        t.Errorf("Name = %q", u.Name)
    }
}
```

`srv.URL` is the real `http://127.0.0.1:<port>` for the test server. Point your client at it. The `defer srv.Close()` shuts it down at test end.

This is the standard way to integration-test code that calls external HTTP APIs without spinning up the real service.

---

## Benchmarks

A benchmark is a `BenchmarkXxx(b *testing.B)` function. The framework chooses `b.N` and times your code; you loop `b.N` times.

```go
func BenchmarkAdd(b *testing.B) {
    for i := 0; i < b.N; i++ {
        Add(2, 3)
    }
}
```

Run:

```bash
go test -bench=. ./math
go test -bench=BenchmarkAdd -benchmem -count=5 ./math
```

Output:

```
BenchmarkAdd-12    1000000000    0.31 ns/op    0 B/op    0 allocs/op
```

- `b.N` is *not* a count you choose — it's what the framework chose to get a stable measurement. Your loop must run cheaply on small `N` and not allocate per-iteration setup.
- `-benchmem` reports allocations. Allocation count is the most actionable number — driving it down usually speeds the code up.
- `-count=N` runs the benchmark N times for variance. Combine with `benchstat` (`golang.org/x/perf/cmd/benchstat`) to compare before/after runs with statistical confidence.
- **Reset the timer** after expensive setup: `b.ResetTimer()`.
- **Stop and start** around per-iteration setup you can't avoid: `b.StopTimer()` / `b.StartTimer()`.

### Go 1.24+: the `b.Loop` form

A newer, less footgun-prone API:

```go
func BenchmarkAdd(b *testing.B) {
    for b.Loop() {
        Add(2, 3)
    }
}
```

`b.Loop()` handles timer management and prevents the compiler from optimising the call away. On a Go 1.24+ module, prefer it.

### Sanity-checking microbenchmarks

The biggest microbenchmark trap is the compiler optimising your code into nothing because the result is unused. Assign to a package-level `var` (the "sink" pattern) to keep the result alive:

```go
var sink int

func BenchmarkAdd(b *testing.B) {
    var s int
    for i := 0; i < b.N; i++ {
        s = Add(2, 3)
    }
    sink = s
}
```

If a benchmark reports nanosecond-per-op times that look implausibly fast, this is usually why.

---

## Coverage

```bash
go test -cover ./...
go test -coverprofile=cover.out ./...
go tool cover -html=cover.out      # opens an HTML report
go tool cover -func=cover.out      # per-function summary
```

Coverage is a guide, not a target. 100% on a `Stringer` adds nothing; 60% on a payment processor with the critical paths covered may be fine. Aim coverage at the places where a regression hurts.

---

## Fuzzing

Go has a built-in fuzzer (1.18+):

```go
func FuzzReverse(f *testing.F) {
    f.Add("hello")
    f.Fuzz(func(t *testing.T, s string) {
        rev  := Reverse(s)
        rev2 := Reverse(rev)
        if rev2 != s {
            t.Errorf("not idempotent: %q -> %q -> %q", s, rev, rev2)
        }
    })
}
```

```bash
go test -fuzz=FuzzReverse -fuzztime=30s ./...
```

The fuzzer mutates seeds and saves any input that triggers a failure (in `testdata/fuzz/`). It's surprisingly effective on parsers, encoders/decoders, and validation logic. Use the seeded inputs as regression tests afterwards.

---

## Example tests (free runnable docs)

A function named `ExampleXxx` is both a test and a documentation sample. If it includes an `// Output:` comment, the framework compares stdout to that.

```go
func ExampleAdd() {
    fmt.Println(Add(2, 3))
    // Output: 5
}
```

This shows up on pkg.go.dev alongside `Add`'s docs, and runs as part of `go test`. Two for one — write at least one for every exported function people might call.

---

## Recap

- Tests are `TestXxx(*testing.T)` in `*_test.go`; run with `go test ./...`.
- Table-driven tests + `t.Run` subtests are the default style; use `t.Parallel()`, `t.Helper()`, `t.Cleanup()`.
- No mocking framework needed — small interfaces + hand-written fakes is the idiom.
- `httptest.NewRecorder` for handler tests; `httptest.NewServer` for fake downstreams.
- Benchmarks use `b.N` (or `b.Loop()` on Go 1.24+); `-benchmem` and `benchstat` are the actual tools.
- Coverage is a guide; fuzzing is excellent for parsers; example tests double as docs.

Next: **Standard Library Tour**.

[← Back to roadmap](../roadmap.md)
