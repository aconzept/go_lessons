# Lesson 1 — Introduction to Go

> For TypeScript developers learning Go.
> Back to [roadmap](../roadmap.md).

This lesson covers:
- Why use Go
- A short history
- Setting up your environment
- Hello, World in Go
- The `go` command

---

## Why use Go

If you write TypeScript, you already know the value of a type system. Go takes that further in a few directions:

| Concern | TypeScript | Go |
|---|---|---|
| Runtime | Node / browser (interpreted, GC) | Compiled to a single native binary, GC |
| Types | Structural, erased at runtime | Structural for interfaces, nominal otherwise, **kept at runtime** |
| Concurrency | `async`/`await`, event loop, single-threaded | Goroutines + channels, true parallelism |
| Package manager | `npm` / `pnpm` / `yarn` | Built-in: `go mod` |
| Build output | `.js` files + `node_modules` | One static executable, no dependencies on host |
| Null safety | `null` / `undefined`, optional chaining | No nulls for value types — every type has a **zero value** |
| Error handling | `throw` / `try`–`catch` | Explicit `error` returns (no exceptions for normal control flow) |

**Pick Go when** you want: fast startup, low memory, easy deployment (one binary), and first-class concurrency. Common use cases: CLI tools, backend services, infra/DevOps tooling (Docker, Kubernetes, Terraform are all written in Go).

**Stay with TS when** you need the browser, a huge ecosystem of UI/SDK packages, or rapid prototyping with dynamic shapes.

---

## A short history

- 2007 — designed at Google by Robert Griesemer, Rob Pike, and Ken Thompson, frustrated with C++ build times and complexity.
- 2009 — open-sourced.
- 2012 — Go 1.0; backwards-compatibility promise for the language.
- 2018 — Go modules (the `go mod` system replacing `GOPATH`-only workflows).
- 2022 — Go 1.18 introduced **generics**.
- Today — stewarded by Google, with regular six-month releases.

The language is deliberately small. There's roughly one idiomatic way to do most things, which makes reading other people's Go code a lot easier than reading other people's TypeScript.

---

## Setting up the environment

### 1. Install Go

- **macOS:** `brew install go`
- **Linux (Arch):** `sudo pacman -S go`
- **Linux (Debian/Ubuntu):** prefer the tarball from <https://go.dev/dl/> — distro packages are often old.
- **Windows:** installer from <https://go.dev/dl/>.

Verify:

```bash
go version
# go version go1.23.x linux/amd64
```

### 2. Editor

VS Code with the official **Go** extension (by Google) is the easiest start. Coming from TS, the experience is similar: gopls is to Go what tsserver is to TS — it provides autocomplete, go-to-definition, refactors, and inline errors.

JetBrains users: **GoLand**, or the Go plugin for IntelliJ.

### 3. Workspace

Unlike the old `GOPATH` days, modern Go projects live anywhere on disk. A project is just a directory with a `go.mod` file (analogous to `package.json`).

```bash
mkdir hello && cd hello
go mod init example.com/hello
```

This creates `go.mod`:

```
module example.com/hello

go 1.23
```

The module path (`example.com/hello`) is the import path other code would use to import your package. For private projects it can be anything — convention is the repo URL (e.g. `github.com/you/hello`).

---

## Hello, World

Create `main.go`:

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
```

Run it:

```bash
go run .
# Hello, World!
```

### What's different from TS

```ts
// TypeScript
console.log("Hello, World!");
```

```go
// Go
package main          // every file declares its package
import "fmt"          // explicit imports, no auto-import of console
func main() { ... }   // entry point is a function named main
```

Things to notice:

- **`package main`** — a special package name. It tells the compiler this is an executable, not a library. The entry point is `func main()`. Libraries use any other package name and have no `main`.
- **No semicolons** — they exist in the grammar, but the lexer inserts them automatically at line ends (similar to ASI in JS, but stricter and more predictable). In practice: don't write them.
- **Braces matter** — the opening `{` of a function **must** be on the same line as the signature. `func main()\n{` is a syntax error. The compiler depends on this for automatic semicolon insertion.
- **Unused imports/variables are compile errors.** `import "fmt"` that you don't use → build fails. This catches a class of bugs early but surprises TS devs used to ESLint warnings rather than failures.

---

## The `go` command

`go` is the Swiss-army knife — it's `tsc`, `npm`, `node`, `eslint`, and `prettier` rolled into one binary. The ones you'll use daily:

| Command | TS analogue | What it does |
|---|---|---|
| `go run .` | `ts-node` / `tsx` | Compile and run the current package in one step. |
| `go build` | `tsc` | Compile to a binary (no run). |
| `go test ./...` | `jest` / `vitest` | Run tests in all sub-packages. |
| `go fmt ./...` | `prettier --write .` | Format code. **No config** — there's one official style. |
| `go vet ./...` | `eslint` (lite) | Catch common mistakes the compiler doesn't. |
| `go mod tidy` | `npm prune` + `npm install` | Add missing and remove unused module dependencies. |
| `go doc fmt.Println` | hovering in editor | Show docs for a symbol from the CLI. |
| `go version` | `node -v` / `tsc -v` | Print Go version. |

Try them now:

```bash
go run .          # Hello, World!
go build          # produces ./hello (or hello.exe)
./hello           # Hello, World!
go fmt ./...      # nothing to fix — it's already formatted
```

The fact that `go build` produces a single static binary you can `scp` to a server and run, with no runtime to install, is one of Go's signature wins over Node.

---

## Recap

- Go is compiled, statically typed, garbage-collected, and small by design.
- A Go project is a directory with a `go.mod`. `package main` + `func main` = executable.
- The `go` toolchain replaces most of the npm/tsc/eslint/prettier stack with one command.
- Formatting and unused-symbol handling are enforced by the compiler/toolchain — no debates.

Next up: **Variables & Constants** — `var`, `:=`, zero values, `const`, and `iota`.

[← Back to roadmap](../roadmap.md)
