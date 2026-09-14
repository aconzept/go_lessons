# Learn to become a Go developer

> Source: roadmap.sh — Go Roadmap

## Language Basics

### [Introduction to Go](lessons/01-introduction-to-go.md)
- Why use Go
- History of Go
- Setting up the Environment
- Hello World in Go
- `go` command

### [Variables & Constants](lessons/02-variables-and-constants.md)
- `var` vs `:=`
- Zero Values
- `const` and `iota`
- Scope and Shadowing

### [Data Types](lessons/03-data-types.md)
- **Numeric Types**
  - Integers (Signed, Unsigned)
  - Floating Points
  - Complex Numbers
- Boolean
- Runes
- **Strings**
  - Raw String Literals
  - Interpreted String Literals
- Type Conversion

### [Commands & Docs](lessons/04-commands-and-docs.md)

---

## Composite Types

### [Arrays & Slices](lessons/05-arrays-and-slices.md)
- Capacity and Growth
- `make()`
- Slice to Array Conversion
- Array to Slice Conversion

### Strings
*(covered in [Data Types](lessons/03-data-types.md))*

### [Maps](lessons/06-maps.md)
- Comma-Ok Idiom

### [Structs](lessons/07-structs.md)
- Struct Tags & JSON
- Embedding Structs

---

## [Loops](lessons/08-loops-and-conditionals.md)
- `for` loop
- `for range`
- Iterating Maps
- Iterating Strings
- `break`
- `continue`
- `goto` (discouraged)

## [Conditionals](lessons/08-loops-and-conditionals.md)
- `if`
- `if-else`
- `switch`

---

## [Functions](lessons/09-functions.md)
- Functions Basics
- Variadic Functions
- Multiple Return Values
- Anonymous Functions
- Closures
- Named Return Values
- Call by Value

## [Pointers](lessons/10-pointers.md)
- Pointers Basics
- Pointers with Structs
- With Maps & Slices
- Get a Brief Overview
- Memory Management
- Garbage Collection

---

## Methods and Interfaces

### [Methods](lessons/11-methods.md)
- Methods vs Functions
- Pointer Receivers
- Value Receivers

### [Interfaces](lessons/12-interfaces.md)
- Interfaces Basics
- Empty Interfaces
- Embedding Interfaces
- Type Assertions
- Type Switch

### [Generics](lessons/13-generics.md)
- Why Generics?
- Generic Functions
- Generic Types / Interfaces
- Type Constraints
- Type Inference

---

## [Code Organization](lessons/14-code-organization.md)

### Modules & Dependencies
- `go mod init`
- `go mod tidy`
- `go mod vendor`

### Packages
- Package Import Rules
- Using 3rd Party Packages
- Publishing Modules

---

## [Error Handling](lessons/15-error-handling.md)
- Error Handling Basics
- `error` interface
- `errors.New`
- `fmt.Errorf`
- Wrapping/Unwrapping Errors
- Sentinel Errors
- `panic` and `recover`
- Stack Traces & Debugging

---

## Concurrency

### [Goroutines & Channels (Part 1)](lessons/16-concurrency-goroutines-and-channels.md)
- Goroutines
- Channels
- Select Statement
- Buffered vs Unbuffered

### [`sync`, Worker Pools, `context` (Part 2)](lessons/17-concurrency-sync-and-context.md)
- Worker Pools
- `sync` Package — Mutexes, WaitGroups, Once
- `context` Package — Deadlines & Cancellations, Common Usecases

### [Patterns & Race Detection (Part 3)](lessons/18-concurrency-patterns-and-race-detection.md)
- Concurrency Patterns — fan-in, fan-out, pipeline
- `errgroup`
- Race Detection

---

## [Testing & Benchmarking](lessons/19-testing-and-benchmarking.md)
- `testing` package basics
- Table-driven Tests
- Mocks and Stubs
- `httptest` for HTTP Tests
- Benchmarks
- Coverage

---

## [Standard Library](lessons/20-standard-library.md)
- I/O & File Handling
- `flag`, `time`, `encoding/json`
- `os`, `bufio`, `slog`, `regexp`
- `go:embed` for embedding

---

## Ecosystem & Popular Libraries

### Building CLIs
- Cobra
- urfave/cli
- bubbletea

### Web Development
- `net/http` (standard)
- **Frameworks (Optional):** gin, echo, fiber, beego

### gRPC & Protocol Buffers

### ORMs & DB Access
- pgx
- GORM

### Logging
- Zerolog
- Zap

### Realtime Communication
- Melody
- Centrifugo

---

## Go Toolchain and Tools

### Core Go Commands
`go run`, `go build`, `go install`, `go fmt`, `go mod`, `go test`, `go clean`, `go doc`, `go version`

### Code Quality and Analysis
- `go vet`
- `goimports`
- **Linters:** revive, staticcheck, golangci-lint
- **Security:** govulncheck

### Code Generation / Build Tags
- `go generate`
- Build Tags

### Performance and Debugging
- pprof
- trace
- Race Detector

### Deployment & Tooling
- Building Executables
- Cross-compilation

---

## Advanced Topics
- Memory Mgmt. in Depth
- Escape Analysis
- Reflection
- Unsafe Package
- Build Constraints & Tags
- CGO Basics
- Compiler & Linker Flags
- Plugins & Dynamic Loading

---

## Practice

Hands-on companions to the lessons above — setup and code you run, rather than concepts you read.

### [Docker & VS Code Debugging](practice/01-docker-and-vscode-debugging.md)
- Debug image that runs your binary under Delve
- `launch.json` for local F5 and for attaching to a container
- Breakpoints, goroutine inspection, and the usual gotchas

### [HTTP Servers with `net/http`](practice/02-http-server.md)
- Handlers, `ServeMux` routing patterns (Go 1.22+), middleware
- JSON request/response, server timeouts, graceful shutdown
- A complete worked service

### [SQL with `database/sql`](practice/03-sql.md)
- The driver package `database/sql` always needs, and why there's no stdlib ORM
- Connection pooling, `Query`/`QueryRow`/`Exec`, scanning, `NULL` handling
- Parameterised queries and transactions

---

*Visit roadmap.sh for the interactive version and related roadmaps: Backend, DevOps, Docker, Kubernetes.*
