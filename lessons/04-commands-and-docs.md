# Lesson 4 — Commands & Docs

> For TypeScript developers learning Go.
> Back to [roadmap](../roadmap.md).

This lesson covers:
- The `go` subcommands you'll use beyond `go run` / `go build`
- `go env` and environment variables that actually matter
- How Go documents code (doc comments, `go doc`, pkg.go.dev)

Lesson 1 introduced the everyday `go` commands. This is the fuller reference — useful as something to come back to.

---

## Command cheat sheet

### Build & run

| Command | Notes |
|---|---|
| `go run .` | Compile + run the package in the current directory. Like `tsx`/`ts-node`. |
| `go run ./cmd/server` | Run a specific sub-package (multi-binary repos do this). |
| `go build` | Compile to a binary in the current dir. Name = directory name. |
| `go build -o bin/server ./cmd/server` | Common in CI: pick output path and target. |
| `go install ./cmd/server` | Build and copy the binary to `$GOBIN` (often `~/go/bin`). The Go equivalent of `npm i -g`. |

### Modules & dependencies

| Command | TS analogue | Notes |
|---|---|---|
| `go mod init <module-path>` | `npm init` | Creates `go.mod`. |
| `go get example.com/pkg@v1.2.3` | `npm install pkg@1.2.3` | Add or upgrade a dependency. Pin or `@latest`. |
| `go get example.com/pkg@none` | `npm uninstall pkg` | Remove. |
| `go mod tidy` | `npm prune` + install | Add anything imported, drop anything not. Run before committing. |
| `go mod download` | `npm ci` | Populate the module cache (CI). |
| `go mod vendor` | committing `node_modules` | Copy deps into `./vendor/`. Optional; some shops require it. |
| `go mod graph` | `npm ls` | Print the dep graph. |

`go.mod` is `package.json`. `go.sum` is `package-lock.json` (checksums). Commit both.

### Test, format, lint

| Command | TS analogue | Notes |
|---|---|---|
| `go test ./...` | `jest` / `vitest` | Run every test in the module. `./...` is "this dir and all sub-packages". |
| `go test -run TestFoo -v ./pkg` | `jest -t "Foo"` | Run one test, verbose. |
| `go test -race ./...` | — | Run with the race detector. Use in CI for any concurrent code. |
| `go test -cover ./...` | `jest --coverage` | Coverage. |
| `go fmt ./...` | `prettier --write .` | Format. No config. |
| `go vet ./...` | `eslint` (subset) | Catches common mistakes the compiler misses. |
| `gofmt -d .` | `prettier --check .` | Show diffs without writing. Used in CI gating. |

There's no "official" linter, but **`golangci-lint`** is the de facto standard meta-linter — it bundles dozens of checkers and is what most teams pin in CI.

### Info

| Command | What it tells you |
|---|---|
| `go version` | Toolchain version. |
| `go env` | Every Go-related env var the toolchain sees. Run this once. |
| `go env GOPATH GOBIN GOMODCACHE` | Specific vars. |
| `go list ./...` | Every package in the current module. |
| `go list -m all` | Every module (direct + transitive). |
| `go doc fmt.Println` | Print docs for one symbol. (More below.) |

---

## `go env` and the variables that matter

Run `go env` once and skim it. The handful worth knowing:

| Variable | What it is |
|---|---|
| `GOPATH` | Where modules and binaries are cached. Default `~/go`. |
| `GOBIN` | Where `go install` puts binaries. Default `$GOPATH/bin`. Add to your `$PATH`. |
| `GOOS` / `GOARCH` | Target OS/arch when building. Cross-compile with `GOOS=linux GOARCH=arm64 go build`. |
| `GOMODCACHE` | Where downloaded modules live. |
| `GOPROXY` | Module proxy. Default `https://proxy.golang.org`; set to `direct` to bypass, or to an internal proxy in enterprise. |
| `GOPRIVATE` | Comma-separated glob of module paths *not* to fetch via the public proxy (private repos). |
| `CGO_ENABLED` | `1` (default) or `0`. Set to `0` for fully static binaries you can drop into a scratch container. |

You write env vars before the command, same as in a shell session:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/server ./cmd/server
```

That's the recipe for "a static Linux binary I can `COPY` into a `FROM scratch` Dockerfile" — one of Go's signature deployment wins.

---

## Documentation

Go takes docs seriously. There is one convention, the toolchain understands it, and it powers both your editor's hover tooltip and the public site at <https://pkg.go.dev>.

### Doc comments

A "doc comment" is a `//` comment **immediately above** an exported declaration. The first sentence should start with the symbol name.

```go
// Package mathx provides small numerical helpers used across services.
package mathx

// Clamp returns v constrained to the closed interval [lo, hi].
// If lo > hi the result is undefined.
func Clamp(v, lo, hi int) int { ... }

// User represents an authenticated end user.
//
// Zero-value Users are not valid — construct with [NewUser].
type User struct { ... }
```

Notes:
- The convention is **terse, declarative prose** — not JSDoc-style `@param`/`@returns` annotations. Types are already in the signature; don't repeat them.
- `[Name]` in a doc comment becomes a link to that symbol on pkg.go.dev.
- A `// Package foo ...` comment above `package foo` documents the package itself. Put it on one file per package (often `doc.go`).
- Exported = starts with an uppercase letter. Unexported names don't need (and shouldn't have) doc comments unless the *why* is non-obvious.

This is the inverse of typical TS practice, where unexported helpers get lots of TSDoc. In Go, the bar is: "would a stranger reading this on pkg.go.dev be confused without a comment?"

### `go doc`

`go doc` reads those comments and prints them.

```bash
go doc fmt                    # package docs + symbol list
go doc fmt.Println            # one function
go doc -src fmt.Println       # function + its source
go doc ./internal/mathx       # docs for a package in your own repo
go doc ./internal/mathx Clamp # one symbol in your own repo
```

Faster than alt-tabbing to a browser, and works in CI containers with no network.

### pkg.go.dev

<https://pkg.go.dev> is the public docs site. Every public module — including yours, the moment you push a tag to a public repo — is auto-indexed. You don't host anything. The site renders the same doc comments `go doc` shows, plus a "Examples" section if you write [example tests](https://pkg.go.dev/testing#hdr-Examples).

For private code, run a local docs server:

```bash
go install golang.org/x/pkgsite/cmd/pkgsite@latest
pkgsite -open .
```

This boots a browser tab showing your module's docs from local source.

### Examples that run

A function in a `_test.go` file named `ExampleClamp` is both a test (its `Output:` comment is asserted) **and** a code sample that pkg.go.dev embeds in the docs:

```go
func ExampleClamp() {
    fmt.Println(mathx.Clamp(15, 0, 10))
    // Output: 10
}
```

This is unique to Go — it solves the "docs out of sync with code" problem at the toolchain level. The first time you ship a library, write a couple.

---

## Recap

- `go` is one tool covering build, run, test, format, vet, deps, docs, and cross-compile.
- `go env` lists everything the toolchain reads from the environment. The cross-compile and `CGO_ENABLED=0` recipes are worth memorising.
- Doc comments are just `//` above exports; `go doc` and pkg.go.dev pick them up automatically. Example tests double as runnable docs.

Next: **Composite Types — Arrays & Slices**.

[← Back to roadmap](../roadmap.md)
