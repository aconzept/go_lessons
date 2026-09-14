# Lesson 14 — Code Organization

> Back to [roadmap](../roadmap.md).

This lesson covers:
- Packages: what they are, how files relate
- Imports — paths, names, aliases, blank imports
- Modules: `go.mod`, `go.sum`, versioning, `replace`
- Adding, upgrading, and removing third-party packages
- The `internal/` rule and how to design package boundaries
- Publishing a module

Go's organisation is two layers: **packages** (a directory of files that compile together) and **modules** (a versioned bundle of packages). Both are simple, but they have a few rules that will save you time once you know them.

---

## Packages

### One package per directory

Every `.go` file in a directory belongs to the same package and declares it on the first line:

```go
package billing
```

The package name is conventionally the last segment of the import path, lowercase, short. `package billing`, not `package BillingService`. Tests in the same directory either use the same package (for white-box testing) or `package billing_test` (for black-box).

### Exported vs unexported

The capitalisation rule, again: a name starting with an uppercase letter is exported from the package; lowercase names are package-private.

```go
package billing

func Charge(...) { ... }     // visible to importers
func computeFee(...) { ... } // package-private
```

There are no `private`/`public` keywords. Files in the same package can see each other's lowercase names freely.

### `init()`

Every package can have one or more `init()` functions — they run once when the package is first loaded, before `main` and before any other code in that package executes:

```go
func init() {
    // register a driver, parse a baked-in template, etc.
}
```

Use sparingly. They make startup order harder to reason about. The common legitimate uses are registering drivers (`sql.Register`), checking baked-in invariants, and setting up package-level singletons.

### `main`

The special package — the one that produces an executable. It must contain `func main()`. A typical multi-binary repo layout:

```
myproj/
├── go.mod
├── cmd/
│   ├── server/    main.go   ← package main
│   └── worker/    main.go   ← package main
├── internal/
│   ├── billing/   ...       ← package billing
│   └── store/     ...       ← package store
└── pkg/                     ← (optional) packages intended for outside use
```

`cmd/<binary>/main.go` is a strong convention worth adopting.

---

## Imports

```go
import (
    "fmt"                       // standard library
    "strings"

    "github.com/yourorg/proj/internal/billing"  // project-internal
    "go.uber.org/zap"                            // third-party
)
```

`gofmt`/`goimports` will group imports as: stdlib, then everything else, separated by blank lines. Most teams add a third group for project-internal imports — that's a `goimports -local <module-path>` setting.

### Aliases and the blank import

```go
import (
    f "fmt"             // alias — refer as f.Println
    . "math"            // dot import — names without prefix (avoid; only in tests)
    _ "image/png"       // blank import — runs init() for side effects, no name imported
)
```

- **Alias** when two imports would otherwise collide, or to shorten a long name. Don't alias for fun.
- **Dot imports** dump everything into your namespace and are universally regretted. The one exception is occasionally in tests.
- **Blank imports** are how driver-style registration works:
  ```go
  import _ "github.com/lib/pq"   // registers a "postgres" driver with database/sql
  ```

### Cyclic imports are forbidden

If package `a` imports `b`, then `b` cannot import `a`, directly or transitively. The compiler will refuse. This is annoying once, then forces you to design a cleaner layering — usually by moving the shared types into a third, lower package.

### Imports must be used

Unused imports are a compile error, same as unused variables. `goimports` saves the day here: it auto-adds and auto-removes imports as you save.

---

## Modules

A module is a tree of packages versioned together. It is defined by a `go.mod` file at its root:

```
module github.com/yourorg/proj

go 1.23

require (
    github.com/google/uuid v1.6.0
    go.uber.org/zap        v1.27.0
)
```

- `module <path>` — the canonical import path of the root. By convention this is the VCS URL, even before you push it (`github.com/you/proj`). For private projects use whatever you like, but the VCS URL is what `go get` later needs.
- `go 1.23` — the **language version** the module targets. It enables features introduced in that version (e.g., per-iteration loop variables in 1.22, the `slices` package in 1.21). It's *not* a minimum toolchain version.
- `require` — direct + transitive dependencies (transitives are pruned in modern modules).

`go.sum` holds checksums for every module version in the build graph. Commit it. CI uses it to verify reproducible builds.

### Versioning

Versions are SemVer with a leading `v`: `v1.6.0`, `v0.4.2`. A version `v2.0.0` or later is a **major version bump** and requires the module path to end in `/v2`, `/v3`, etc.:

```
require github.com/foo/bar/v2 v2.1.0
```

This is why you sometimes see `/v2` in import paths. The version is part of the path, so `v1` and `v2` of the same library can coexist in one build.

Pre-1.0 versions (`v0.x.y`) carry no compatibility guarantees — moving from `v0.4.0` to `v0.5.0` may break things by design.

### `replace` and `exclude`

```
replace github.com/foo/bar => ../bar              // use a local copy
replace github.com/foo/bar v1.2.3 => github.com/myfork/bar v1.2.4
```

`replace` is for local development against an unpublished dependency, or for pinning to a fork. Don't leave `replace` directives pointing to local paths in committed code unless every contributor has the same checkout layout.

---

## Adding, upgrading, removing dependencies

```bash
go get github.com/google/uuid               # add (latest)
go get github.com/google/uuid@v1.5.0        # pin
go get github.com/google/uuid@latest        # upgrade
go get -u ./...                             # upgrade all (minor/patch)
go get github.com/google/uuid@none          # remove
go mod tidy                                 # reconcile go.mod with imports
```

The most important command here is `go mod tidy`. Run it before committing — it adds anything you imported and didn't declare, and removes anything you declared and don't import. CI should run `go mod tidy && git diff --exit-code go.mod go.sum` to enforce that the files stay in sync with the code.

---

## `internal/` — the only access-control directory

A package at any path containing `/internal/` is importable only from packages rooted at the directory *above* that `internal/`. So:

```
myproj/internal/billing       importable by  myproj/...
myproj/internal/billing       NOT importable by  github.com/other/project
```

This is enforced by the compiler. It's the cleanest way to expose a clear public API while keeping the rest of your code free to refactor. The rule of thumb: **put everything in `internal/` by default**; promote a package out of `internal/` (to `pkg/`, say) only when you're explicitly choosing to support it as a stable public API.

### Designing package boundaries

A few heuristics that hold up:

- **Package name describes what it provides**, not what it contains. `billing`, `auth`, `store` — not `utils`, `helpers`, `common`. A "utils" package is a smell that there's a missing real domain concept.
- **Avoid `util.go` / `helpers.go`**. Either the function belongs to a real package, or it's small enough to inline.
- **Don't paper over a cyclic import with an interface in a new third package** unless that third package is named after a *real* concept. Otherwise you've just hidden the design problem.
- **One package, one responsibility.** If a package has 60 exported symbols, it's probably two packages.

---

## Publishing a module

To make a module installable by others:

1. Push to a public repo whose URL matches the `module` directive in `go.mod`.
2. Tag a release: `git tag v0.1.0 && git push --tags`.
3. Optionally trigger pkg.go.dev to fetch:
   ```
   curl https://proxy.golang.org/github.com/you/proj/@v/v0.1.0.info
   ```
   (It'll be picked up automatically the first time anyone `go get`s it.)

Other people can now:

```bash
go get github.com/you/proj@v0.1.0
```

That's it — no registry account, no `npm publish` step. The version is just a git tag; pkg.go.dev reads `go.mod` directly from the repo.

For private modules, set `GOPRIVATE` so the toolchain doesn't try the public proxy:

```bash
go env -w GOPRIVATE=github.com/yourorg/*
```

And make sure git can authenticate to that host (SSH keys, or `~/.netrc` for HTTPS).

---

## Recap

- A package is a directory; the package name lives at the top of every `.go` file.
- Uppercase = exported, lowercase = package-private. No cyclic imports.
- A module is a versioned bundle of packages with `go.mod` + `go.sum` at its root. SemVer; major versions ≥ 2 carry `/vN` in the import path.
- `go get` adds/updates/removes; `go mod tidy` keeps the files honest. Commit both `go.mod` and `go.sum`.
- `internal/` is real, compiler-enforced access control — default new code there.
- Publishing = push + tag. No central registry.

Next: **Error Handling**.

[← Back to roadmap](../roadmap.md)
