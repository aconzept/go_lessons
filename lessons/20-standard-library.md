# Lesson 20 — Standard Library Tour

> Back to [roadmap](../roadmap.md).

This lesson covers:
- `io` and the Reader/Writer model
- `os` and file I/O
- `bufio` for buffered streams
- `flag` for command-line flags
- `time` — durations, timers, formatting
- `encoding/json` in real use
- `slog` — the modern structured logger
- `regexp`
- `go:embed` for embedding files into your binary

Go's standard library is broad and quietly excellent. Most production Go services need very few third-party packages — most of what you reach for is here. This lesson walks through the parts you'll actually use day-to-day, with enough on each to recognise the idioms.

---

## `io` — the Reader/Writer model

`io` defines tiny interfaces that everything else builds on. The two you'll see constantly:

```go
type Reader interface { Read(p []byte) (n int, err error) }
type Writer interface { Write(p []byte) (n int, err error) }
```

Files, HTTP bodies, gzip streams, network connections, byte buffers, even strings — all of these satisfy these interfaces. As a result, code that takes a `Reader` works against any of them.

```go
import "io"

n, err := io.Copy(dst, src)       // streams src into dst until EOF
data, err := io.ReadAll(src)      // read everything into memory (small inputs!)
```

Useful adapters:

| Adapter | Does |
|---|---|
| `strings.NewReader(s)` | Treat a string as an `io.Reader`. |
| `bytes.NewReader(b)` | Same, from a `[]byte`. |
| `bytes.Buffer` | A mutable `[]byte` that's both a Reader and a Writer. |
| `io.Discard` | A `Writer` that throws everything away (for benchmarking, headers-only HTTP reads). |
| `io.MultiReader` / `io.MultiWriter` | Concatenate or tee. |

The `EOF` convention: `Read` returns `io.EOF` (a sentinel error) when there's nothing left. Use `errors.Is(err, io.EOF)` to check; treat EOF as "done", not "failure".

---

## `os` — files and the process

### Reading a file, the easy way

```go
data, err := os.ReadFile("/etc/app.yaml")   // []byte, error
```

For small files. Don't use this on a 4 GB log.

### Reading a file, the streaming way

```go
f, err := os.Open("big.log")
if err != nil { return err }
defer f.Close()

// f is an *os.File, which satisfies io.Reader
```

### Writing a file

```go
err := os.WriteFile("out.txt", []byte("hello\n"), 0o644)   // whole file
```

The third arg is the Unix permission. Use the `0o` prefix (octal) — it's the Go-idiomatic form.

For streaming writes:

```go
f, err := os.Create("out.txt")      // truncates if it exists
defer f.Close()
fmt.Fprintln(f, "hello")
```

### Process odds and ends

```go
os.Args              // []string — program args including [0] = program name
os.Getenv("PORT")    // env var, "" if missing
os.LookupEnv("X")    // (value, ok) — distinguishes empty from unset
os.Stdin/Stdout/Stderr
os.Exit(2)           // immediate; does NOT run deferred functions
```

`os.Exit` is sharp: it bypasses `defer`. If you've opened files or started goroutines, prefer returning an error up to `main` and exiting cleanly there.

---

## `bufio` — buffer your streams

Reading or writing one byte at a time straight to disk or the network is slow. `bufio` adds a buffer in front.

### Buffered reader / scanner

```go
f, _ := os.Open("lines.txt")
defer f.Close()

sc := bufio.NewScanner(f)
for sc.Scan() {
    line := sc.Text()
    process(line)
}
if err := sc.Err(); err != nil {
    return err
}
```

`bufio.Scanner` splits an input into lines by default, calling `sc.Bytes()` or `sc.Text()` per chunk. You can change the split function (`sc.Split(bufio.ScanWords)`, for instance).

One gotcha: `Scanner` has a maximum token size (default 64 KB). For very long lines, raise it with `sc.Buffer(...)` or drop to `bufio.Reader.ReadString('\n')`.

### Buffered writer

```go
w := bufio.NewWriter(os.Stdout)
defer w.Flush()                       // critical — flush before close

fmt.Fprintln(w, "one")
fmt.Fprintln(w, "two")
```

If you forget `w.Flush()`, the last bit of output silently disappears when the program exits. Pair every `NewWriter` with a `defer Flush()` immediately.

---

## `flag` — command-line flags

Built-in flag parser, perfectly fine for the 90% case:

```go
import "flag"

var (
    addr    = flag.String("addr",    ":8080", "listen address")
    workers = flag.Int   ("workers",       4, "number of workers")
    debug   = flag.Bool  ("debug",     false, "enable debug logging")
)

func main() {
    flag.Parse()
    fmt.Printf("listening on %s with %d workers\n", *addr, *workers)
}
```

```bash
./app -addr :9000 -workers 8 -debug
./app -h          # auto-generated help
```

Notes:

- Flag variables are pointers — `*addr` to read.
- The `flag.StringVar(&addr, ...)` form lets you provide your own variable instead of getting a pointer back. Pick one style and stick to it.
- Reach for `spf13/cobra` (or `urfave/cli`) when you need subcommands (`mytool deploy ...`, `mytool config ...`). `flag` doesn't do those.

---

## `time`

Three things to understand: `time.Time`, `time.Duration`, and the formatting conventions.

### Time and duration

```go
now := time.Now()
later := now.Add(2 * time.Hour)
elapsed := time.Since(start)            // shorthand for time.Now().Sub(start)
deadline := time.Now().Add(5 * time.Second)

if time.Until(deadline) < 0 { ... }
```

Durations are typed integers (`type Duration int64`) measured in nanoseconds. You write them with constants:

```go
500 * time.Millisecond
3   * time.Second
24  * time.Hour
```

`Duration` has methods like `Seconds() float64` for printing or arithmetic in seconds.

### Sleeping, timers, tickers

```go
time.Sleep(time.Second)                          // simple

t := time.NewTimer(2 * time.Second)
<-t.C                                            // fires once
t.Stop()                                         // release if you didn't wait

tk := time.NewTicker(500 * time.Millisecond)
defer tk.Stop()
for range tk.C {
    work()
}
```

`time.After(d)` is a one-shot channel that's allocator-heavy in tight loops — prefer `NewTimer` + `Reset` there.

### Formatting

Go's formatting reference is the most-cited piece of trivia in the language:

```
Mon Jan 2 15:04:05 MST 2006
```

That exact date is the *template*. To format the current time as `2006-01-02 15:04:05`:

```go
fmt.Println(time.Now().Format("2006-01-02 15:04:05"))
```

Yes, those are literal digits in the template — `2006` is the year, `01` is the month, etc. It's weird but consistent. For most needs there are pre-defined constants:

```go
time.RFC3339              // "2006-01-02T15:04:05Z07:00"
time.RFC3339Nano
time.DateTime             // "2006-01-02 15:04:05" (Go 1.20+)
time.DateOnly             // "2006-01-02"
time.Kitchen              // "3:04PM"

time.Now().Format(time.RFC3339)
```

Parsing uses the same template:

```go
t, err := time.Parse(time.RFC3339, "2026-05-19T12:00:00Z")
```

Timezones are a real concept here — `time.Time` carries a `Location`. Be explicit:

```go
loc, _ := time.LoadLocation("America/New_York")
t := time.Date(2026, 5, 19, 9, 0, 0, 0, loc)
```

---

## `encoding/json`

### Marshal / Unmarshal

```go
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email,omitempty"`
}

b, err := json.Marshal(User{ID: 1, Name: "Ada"})
// b = `{"id":1,"name":"Ada"}` — no email because omitempty + zero

var u User
err = json.Unmarshal(b, &u)            // note: pointer
```

Struct tags (covered in lesson 7) drive the field names. `omitempty` skips the field when it equals the zero value. `-` excludes it entirely.

### Streaming with Encoder/Decoder

For HTTP handlers and other stream contexts:

```go
// Server response
w.Header().Set("Content-Type", "application/json")
_ = json.NewEncoder(w).Encode(payload)

// Server request body
var req CreateRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    http.Error(w, "bad json", 400); return
}
```

`Encoder`/`Decoder` work directly on `io.Writer`/`io.Reader` — no intermediate `[]byte`. Important for large bodies and streaming endpoints.

### Unknown shapes

When you don't know the shape upfront, decode into a `map[string]any` or `[]any`:

```go
var raw map[string]any
_ = json.Unmarshal(b, &raw)

// Type-assert as you go
if n, ok := raw["count"].(float64); ok { ... }   // numbers come back as float64
```

JSON numbers come out as `float64` unless you tell the decoder otherwise (`Decoder.UseNumber()`). Booleans as `bool`, strings as `string`, objects as `map[string]any`, arrays as `[]any`, null as `nil`.

### Real-world tips

- **Always check the error from `Unmarshal`.** A malformed body or wrong-shaped JSON is an error, not a panic.
- **Encoder adds a trailing newline.** Marshal does not. Tests sometimes trip on this.
- **JSON in production traffic is hot path code.** If you're profiling and see `encoding/json` near the top, that's normal; switch to a generated decoder (`easyjson`, `sonic`) only after measuring.

---

## `slog` — structured logging

Go 1.21 added a structured logger to the standard library. Use it for new code; legacy code might still be on `log.Println` or a third-party logger (zap, zerolog) — those still work fine but `slog` is now the default.

```go
import "log/slog"

slog.Info("user logged in",
    "user_id", u.ID,
    "ip",      r.RemoteAddr)
// {"time":"...","level":"INFO","msg":"user logged in","user_id":42,"ip":"..."}
```

The message is a constant string; everything that's data is a key/value pair. This is what makes the logs structured — easy to query in any log backend.

### Picking a handler

```go
// Default: text, to stderr
slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

// JSON for production
slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
})))
```

### Adding context with `With`

```go
log := slog.With("request_id", id)
log.Info("handling")           // every log gets request_id
log.Error("failed", "err", err)
```

This is the structured equivalent of "pass the logger down with extra fields baked in". For per-request loggers in HTTP middleware, this is the idiomatic shape.

### Levels

`slog.LevelDebug`, `LevelInfo`, `LevelWarn`, `LevelError`. The handler decides which ones to emit; default is Info and above.

---

## `regexp`

Go's regex is the **RE2** engine — no backtracking, linear time in input size, no catastrophic-regex DoS. The trade-off: no backreferences and no lookarounds. Most real patterns don't need them.

```go
import "regexp"

re := regexp.MustCompile(`(\w+)@(\w+\.\w+)`)

re.MatchString("ada@example.com")              // true
re.FindStringSubmatch("write ada@example.com to her")
// ["ada@example.com", "ada", "example.com"]
```

Two compilers:

- `regexp.Compile(s)` returns `(*Regexp, error)` — use when the pattern is user input.
- `regexp.MustCompile(s)` panics on a bad pattern — fine at init time with a literal pattern (most cases).

Compile **once**, at package level, then reuse:

```go
var emailRe = regexp.MustCompile(`...`)
```

Recompiling per call is a common perf footgun.

Useful methods to know exist:

- `MatchString`, `FindString`, `FindAllString`
- `FindStringSubmatch` (returns groups), `FindAllStringSubmatch`
- `ReplaceAllString`, `ReplaceAllStringFunc`
- `Split`

If a pattern works in Perl/JS but errors here, it likely uses a feature RE2 doesn't support. Restate it without lookaround.

---

## `go:embed`

The compiler can embed files into the binary at build time:

```go
import _ "embed"

//go:embed templates/index.html
var indexHTML string

//go:embed static
var staticFS embed.FS

func main() {
    http.Handle("/", http.FileServer(http.FS(staticFS)))
    http.HandleFunc("/index", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprint(w, indexHTML)
    })
    _ = http.ListenAndServe(":8080", nil)
}
```

Rules:

- The `//go:embed` directive is a magic comment **directly above** a `var` declaration. No blank lines between.
- The blank import `_ "embed"` is required when you only use the directive (no symbols from the `embed` package).
- The target variable type can be `string`, `[]byte`, or `embed.FS` (for directories / multiple files).
- Patterns are relative to the file containing the directive. Globs work: `//go:embed templates/*`.

This is how Go services ship a single static binary that includes their HTML templates, SQL migrations, and static assets. Replaces `pkger`, `statik`, and friends entirely.

---

## What we skipped on purpose

- `net/http` — covered later in the Ecosystem section (web development).
- `database/sql` — also under Ecosystem.
- `context` — full lesson in concurrency.
- `errors`/`fmt` — full lesson in error handling.
- `sort`, `slices`, `maps`, `cmp` — touched in earlier lessons.
- `strings`, `strconv`, `unicode` — small, self-explanatory once you know they exist.

That leaves the core "system / I/O / data" surface, which is what this lesson covered.

---

## Recap

- `io.Reader`/`io.Writer` is the universal abstraction — everything I/O-shaped satisfies it.
- `os.ReadFile`/`os.WriteFile` for small files; open + `bufio` for streams. Pair `bufio.NewWriter` with `defer Flush()`.
- `flag` for simple CLIs; cobra/urfave for subcommands.
- `time.Duration` is nanoseconds; the formatting template is the magic `2006-01-02 15:04:05`. Be explicit about timezones.
- `encoding/json` covers most cases; struct tags drive field names; numbers decode as `float64` unless told otherwise.
- `slog` is the modern logger — key/value pairs, JSON or text handlers, context with `With`.
- `regexp` is RE2: linear-time, no lookarounds; compile once at init.
- `go:embed` ships files inside the binary — string, []byte, or `embed.FS`.

Next: **Ecosystem & Popular Libraries** — building CLIs, HTTP servers, gRPC, DB access, logging frameworks, and realtime communication.

[← Back to roadmap](../roadmap.md)
