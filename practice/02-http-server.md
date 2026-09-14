# Practice 2 — HTTP Servers with `net/http`

> Back to [roadmap](../roadmap.md).

This lesson covers:
- `http.Handler` and `http.HandlerFunc`
- Routing with `http.ServeMux` — method patterns and wildcards (Go 1.22+)
- Reading the request: path values, query, headers, JSON body
- Writing the response: status codes, JSON, errors
- Middleware, and how to chain it
- `http.Server` configuration — the timeouts you must set
- Graceful shutdown
- Calling other services with `http.Client`
- A complete worked service
- When a framework earns its place

The standard library is a production-grade HTTP server. Most Go services never import a web framework. This lesson is the shape of a real one.

---

## Handlers

Everything in `net/http` is built on one interface:

```go
type Handler interface {
    ServeHTTP(w http.ResponseWriter, r *http.Request)
}
```

One method — exactly the small-interface style from [Lesson 12](../lessons/12-interfaces.md). You rarely implement it directly, because of this adapter:

```go
type HandlerFunc func(http.ResponseWriter, *http.Request)

func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) { f(w, r) }
```

`HandlerFunc` is a named function type with a method on it — the pattern from [Lesson 11](../lessons/11-methods.md). It lets any plain function become a `Handler`:

```go
func hello(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "hi, %s", r.PathValue("name"))
}

var h http.Handler = http.HandlerFunc(hello)
```

### Handlers that need dependencies

A handler can't take extra arguments — the signature is fixed. Two idiomatic ways to give it a database, a logger, or config:

```go
// 1. Method on a struct  (preferred for a group of related handlers)
type Server struct {
    users UserStore
    log   *slog.Logger
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
    u, err := s.users.Find(r.Context(), r.PathValue("id"))
    // ...
}

// 2. Closure returning a handler  (good for one-offs)
func handleHealth(version string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprint(w, version)
    }
}
```

Both are just closures over state. There is no DI container, and none is needed.

---

## Routing with `ServeMux`

Go 1.22 taught the standard mux **methods** and **wildcards**, which is why most services no longer need a third-party router.

```go
mux := http.NewServeMux()

mux.HandleFunc("GET /healthz",           handleHealth)
mux.HandleFunc("GET /users",             s.handleListUsers)
mux.HandleFunc("POST /users",            s.handleCreateUser)
mux.HandleFunc("GET /users/{id}",        s.handleGetUser)
mux.HandleFunc("DELETE /users/{id}",     s.handleDeleteUser)
mux.HandleFunc("GET /files/{path...}",   s.handleFile)   // trailing wildcard: matches the rest
mux.Handle    ("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web"))))
```

Pattern rules worth knowing:

| Pattern | Matches |
|---|---|
| `GET /users` | that method and that exact path |
| `/users` | **any** method on that path |
| `/users/{id}` | one path segment, readable as `r.PathValue("id")` |
| `/files/{path...}` | the remainder of the path, slashes included |
| `/static/` | trailing slash = subtree (prefix) match |
| `/users/{$}` | `/users/` exactly, without the subtree match |

Precedence is **most specific wins**, not registration order — `/users/{id}` loses to `/users/me`. Conflicting patterns that can't be ordered panic at registration, which is a good time to find out.

A `ServeMux` is itself a `Handler`, so you can nest one under a prefix for API versioning:

```go
api := http.NewServeMux()
api.HandleFunc("GET /users", s.handleListUsers)

root := http.NewServeMux()
root.Handle("/v1/", http.StripPrefix("/v1", api))
```

An unmatched request gets a 404 automatically. `r.PathValue` on a name that isn't in the pattern returns `""`, not an error — check it when it matters.

---

## Reading the request

```go
func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")                       // /users/{id}

    limit := r.URL.Query().Get("limit")           // ?limit=20  ("" if absent)
    verbose := r.URL.Query().Has("verbose")       // presence, not value

    auth := r.Header.Get("Authorization")         // canonicalised, case-insensitive
    ctx := r.Context()                            // cancelled when the client disconnects
    ...
}
```

`r.Context()` is the one to internalise: propagate it into every DB call, RPC, and goroutine you spawn ([Lesson 17](../lessons/17-concurrency-sync-and-context.md)). When the client hangs up, your query gets cancelled instead of running to completion for nobody.

### Decoding a JSON body

```go
type createUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20)   // 1 MiB cap — do this

    var req createUserRequest
    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()                      // optional: strict APIs
    if err := dec.Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid JSON body")
        return
    }

    if req.Name == "" {
        writeError(w, http.StatusUnprocessableEntity, "name is required")
        return
    }
    ...
}
```

Three things people skip and later regret:

1. **`http.MaxBytesReader`** — without it, a client can stream you gigabytes and OOM the process.
2. **Validation after decoding.** `Unmarshal` doesn't error on missing fields; they arrive as zero values ([Lesson 7](../lessons/07-structs.md)).
3. **Never `panic` on bad input.** A malformed body is an expected failure, not a bug ([Lesson 15](../lessons/15-error-handling.md)).

You do not need to `defer r.Body.Close()` in a handler — the server does it for you. (You *do* on the client side.)

---

## Writing the response

`http.ResponseWriter` has three methods, and the order you call them matters:

```go
w.Header().Set("Content-Type", "application/json")   // 1. headers first
w.WriteHeader(http.StatusCreated)                    // 2. then status
_ = json.NewEncoder(w).Encode(user)                  // 3. then body
```

Headers set *after* `WriteHeader` are silently ignored. Writing a body without calling `WriteHeader` implies `200 OK`.

Two small helpers pay for themselves immediately:

```go
func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(v); err != nil {
        // Too late to change the status — the header is already out.
        slog.Error("write response", "err", err)
    }
}

func writeError(w http.ResponseWriter, status int, msg string) {
    writeJSON(w, status, map[string]string{"error": msg})
}
```

`http.Error(w, msg, status)` is the stdlib one-liner, but it writes `text/plain` — fine for a health check, wrong for a JSON API.

**Return after writing.** Forgetting `return` after an error response is the single most common `net/http` bug: the handler keeps going and you get `superfluous response.WriteHeader call` in the logs.

---

## Middleware

Middleware is just a function that wraps a `Handler` in another `Handler`. No framework required:

```go
type Middleware func(http.Handler) http.Handler

func Logging(log *slog.Logger) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

            next.ServeHTTP(rec, r)

            log.Info("request",
                "method",  r.Method,
                "path",    r.URL.Path,
                "status",  rec.status,
                "dur_ms",  time.Since(start).Milliseconds())
        })
    }
}

// Capture the status code on its way past.
type statusRecorder struct {
    http.ResponseWriter
    status int
}

func (r *statusRecorder) WriteHeader(code int) {
    r.status = code
    r.ResponseWriter.WriteHeader(code)
}
```

`statusRecorder` embeds `http.ResponseWriter` ([Lesson 7](../lessons/07-structs.md)) so it satisfies the interface for free, and overrides just the one method it cares about.

A panic-recovery middleware is the other one every service wants:

```go
func Recover(log *slog.Logger) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if rec := recover(); rec != nil {
                    log.Error("panic", "err", rec, "stack", string(debug.Stack()))
                    writeError(w, http.StatusInternalServerError, "internal error")
                }
            }()
            next.ServeHTTP(w, r)
        })
    }
}
```

This is exactly the "recover at a process boundary" case from [Lesson 15](../lessons/15-error-handling.md) — one bad request must not take the server down.

Chaining:

```go
func Chain(h http.Handler, mw ...Middleware) http.Handler {
    for i := len(mw) - 1; i >= 0; i-- {   // reverse: first listed runs first
        h = mw[i](h)
    }
    return h
}

handler := Chain(mux, Recover(log), Logging(log))
```

---

## The `http.Server` — and its timeouts

`http.ListenAndServe(":8080", mux)` is fine for a demo and wrong for production, because it leaves every timeout at zero, meaning *infinite*. A slow client can hold a connection open forever.

```go
srv := &http.Server{
    Addr:              ":" + port,
    Handler:           handler,
    ReadHeaderTimeout: 5 * time.Second,    // slowloris defence
    ReadTimeout:       15 * time.Second,
    WriteTimeout:      30 * time.Second,
    IdleTimeout:       60 * time.Second,
    MaxHeaderBytes:    1 << 20,
    ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
}
```

Note `WriteTimeout` covers the whole response — if you stream large files or hold long-poll connections, raise it for those routes or set it to 0 and enforce deadlines per-handler with `context.WithTimeout`.

---

## Graceful shutdown

Containers get `SIGTERM` and then, some seconds later, `SIGKILL`. Between those you want in-flight requests to finish:

```go
func run() error {
    ctx, stop := signal.NotifyContext(context.Background(),
        os.Interrupt, syscall.SIGTERM)
    defer stop()

    srv := &http.Server{ /* as above */ }

    errCh := make(chan error, 1)
    go func() {
        slog.Info("listening", "addr", srv.Addr)
        if err := srv.ListenAndServe(); err != nil &&
            !errors.Is(err, http.ErrServerClosed) {
            errCh <- err
        }
    }()

    select {
    case err := <-errCh:
        return err
    case <-ctx.Done():
        slog.Info("shutting down")
    }

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    return srv.Shutdown(shutdownCtx)
}
```

Details that matter:

- `signal.NotifyContext` turns signals into a normal cancellable context — no signal channel plumbing.
- `ListenAndServe` **always** returns a non-nil error; on a clean `Shutdown` it's `http.ErrServerClosed`, which is not a failure. Check with `errors.Is` ([Lesson 15](../lessons/15-error-handling.md)).
- `Shutdown` stops accepting new connections, then waits for active handlers. The timeout is your cap on how long you'll wait.
- `errCh` is buffered with capacity 1 so the goroutine can exit even if nobody reads it — otherwise you leak it ([Lesson 18](../lessons/18-concurrency-patterns-and-race-detection.md)).

---

## The client side

```go
var client = &http.Client{Timeout: 10 * time.Second}

func FetchUser(ctx context.Context, base string, id int) (*User, error) {
    url := fmt.Sprintf("%s/users/%d", base, id)

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("build request: %w", err)
    }

    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("get %s: %w", url, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("get %s: status %d", url, resp.StatusCode)
    }

    var u User
    if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
        return nil, fmt.Errorf("decode user: %w", err)
    }
    return &u, nil
}
```

Four rules:

1. **Always `defer resp.Body.Close()`** — not closing leaks the connection out of the pool.
2. **Never use `http.DefaultClient` in production.** It has no timeout, so a hung server hangs you forever.
3. **`NewRequestWithContext`**, always — that's what makes the call cancellable.
4. **A non-2xx status is not an `err`.** `client.Do` only errors on transport failures; check `resp.StatusCode` yourself.

Reuse one `*http.Client` for the process. It pools connections; creating one per request throws that away.

---

## A complete service

```go
package main

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "runtime/debug"
    "sync"
    "syscall"
    "time"
)

type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

type Server struct {
    log *slog.Logger

    mu     sync.RWMutex          // guards the map — see Lesson 17
    users  map[int]User
    nextID int
}

func NewServer(log *slog.Logger) *Server {
    return &Server{log: log, users: map[int]User{}, nextID: 1}
}

func (s *Server) Routes() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /healthz",    s.handleHealth)
    mux.HandleFunc("GET /users",      s.handleList)
    mux.HandleFunc("POST /users",     s.handleCreate)
    mux.HandleFunc("GET /users/{id}", s.handleGet)
    return Chain(mux, Recover(s.log), Logging(s.log))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleList(w http.ResponseWriter, _ *http.Request) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    out := make([]User, 0, len(s.users))   // 0-length, not nil: marshals as [] not null
    for _, u := range s.users {
        out = append(out, u)
    }
    writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

    var req struct {
        Name string `json:"name"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid JSON body")
        return
    }
    if req.Name == "" {
        writeError(w, http.StatusUnprocessableEntity, "name is required")
        return
    }

    s.mu.Lock()
    u := User{ID: s.nextID, Name: req.Name}
    s.users[u.ID] = u
    s.nextID++
    s.mu.Unlock()

    w.Header().Set("Location", fmt.Sprintf("/users/%d", u.ID))
    writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
    var id int
    if _, err := fmt.Sscanf(r.PathValue("id"), "%d", &id); err != nil {
        writeError(w, http.StatusBadRequest, "id must be an integer")
        return
    }

    s.mu.RLock()
    u, ok := s.users[id]        // comma-ok — Lesson 6
    s.mu.RUnlock()

    if !ok {
        writeError(w, http.StatusNotFound, "user not found")
        return
    }
    writeJSON(w, http.StatusOK, u)
}

func main() {
    log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    slog.SetDefault(log)

    if err := run(log); err != nil {
        log.Error("server failed", "err", err)
        os.Exit(1)
    }
}

func run(log *slog.Logger) error {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    srv := &http.Server{
        Addr:              ":" + port,
        Handler:           NewServer(log).Routes(),
        ReadHeaderTimeout: 5 * time.Second,
        ReadTimeout:       15 * time.Second,
        WriteTimeout:      30 * time.Second,
        IdleTimeout:       60 * time.Second,
    }

    ctx, stop := signal.NotifyContext(context.Background(),
        os.Interrupt, syscall.SIGTERM)
    defer stop()

    errCh := make(chan error, 1)
    go func() {
        log.Info("listening", "addr", srv.Addr)
        if err := srv.ListenAndServe(); err != nil &&
            !errors.Is(err, http.ErrServerClosed) {
            errCh <- err
        }
    }()

    select {
    case err := <-errCh:
        return err
    case <-ctx.Done():
        log.Info("shutdown signal received")
    }

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    return srv.Shutdown(shutdownCtx)
}
```

Try it:

```bash
go run ./cmd/server

curl localhost:8080/healthz
curl -X POST localhost:8080/users -d '{"name":"Ada"}'
curl localhost:8080/users/1
curl -i localhost:8080/users/99          # 404 + JSON error body
```

Now put a breakpoint on the first line of `handleCreate`, attach the debugger from [Practice 1](01-docker-and-vscode-debugging.md), and re-run the `POST`. The request freezes mid-handler and you can inspect `req`, `s.users`, and the goroutine the server spawned for that connection.

---

## Testing it

`net/http/httptest` covers this without opening a port — the two patterns are in [Lesson 19](../lessons/19-testing-and-benchmarking.md). The reason `Routes()` is a method returning a `Handler` is precisely so a test can build a `Server`, call `Routes()`, and drive it with `httptest.NewRecorder()` — no `main`, no network, no sleeping.

---

## Do you need a framework?

The stdlib covers routing, middleware, JSON, timeouts, TLS, HTTP/2, and graceful shutdown. Since 1.22 the router does methods and path parameters too, which removed the last common reason to import one.

Reach for `chi` (closest to stdlib, `http.Handler` all the way down), `gin` or `echo` (batteries: binding, validation, rendering) when you specifically want route groups with per-group middleware, built-in request binding/validation, or a large existing team convention. Start with `net/http`; you'll know when something is missing.

Middleware written against `http.Handler` works in `net/http` and `chi` unchanged — another reason to start plain.

---

## Recap

- `http.Handler` is one method; `http.HandlerFunc` adapts plain functions. Give handlers dependencies by making them methods on a struct.
- `ServeMux` since 1.22 handles `"GET /users/{id}"` patterns; read values with `r.PathValue`. Most specific pattern wins.
- Cap request bodies with `MaxBytesReader`, validate after decoding, and always `return` after writing an error.
- Set headers → status → body, in that order.
- Middleware is `func(http.Handler) http.Handler`. Every service wants logging and panic recovery.
- Configure `http.Server` timeouts explicitly; `ListenAndServe`'s defaults are infinite.
- Shut down with `signal.NotifyContext` + `srv.Shutdown(ctx)`; treat `http.ErrServerClosed` as success.
- On the client: one shared `*http.Client` with a timeout, `NewRequestWithContext`, `defer resp.Body.Close()`, and check the status yourself.

Next: **[SQL with `database/sql`](03-sql.md)** — the `UserStore` this lesson's `Server` struct took as a dependency, actually implemented.

[← Back to roadmap](../roadmap.md)
