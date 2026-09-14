# Practice 1 — Docker & VS Code Debugging

> Back to [roadmap](../roadmap.md).

This lesson covers:
- A Dev Container that gives you Go, gopls, and Delve with **nothing installed on the host**
- A `go.work` multi-module workspace, so every practice module under `code/` gets cross-module IntelliSense without a single `replace` directive
- One trivial `launch.json` config — no `host`/`port`/`substitutePath` gymnastics
- The gotchas that cost an afternoon

The goal: press **F5** in VS Code, land on a breakpoint inside a container, and step through with the full Delve toolkit — variables, watches, call stack, and the goroutine list. Your Windows/macOS/Linux host never needs a Go toolchain.

---

## Prerequisites

1. The **Dev Containers** extension (`ms-vscode-remote.remote-containers`).
2. Docker Desktop / Docker Engine.

That's it. Specifically **not** required: Go on the host, Delve on the host, the Go VS Code extension installed at the user level (it gets installed *inside* the container instead — see `devcontainer.json` below).

**Reopen in Container** (`Ctrl+Shift+P` → **Dev Containers: Reopen in Container**) moves the entire extension host — the Go extension, gopls, everything — into the container. There's no host/remote split to configure around: "local" now *is* the container.

---

## The layout

```
go_lessons/
├── go.work                    ← workspace file, ties modules under code/ together
├── code/
│   └── test1/
│       ├── go.mod
│       └── main.go
├── .devcontainer/
│   ├── devcontainer.json
│   └── Dockerfile
└── .vscode/
    └── launch.json
```

Each practice exercise is its own module under `code/`, with its own `go.mod`. `go.work` at the root is what makes them feel like one project instead of a pile of disconnected directories — more on that below.

---

## The container image

```dockerfile
# .devcontainer/Dockerfile
FROM golang:1.27

RUN apt-get update && apt-get install -y --no-install-recommends build-essential
```

That's the whole thing. `build-essential` provides a C compiler, which Delve needs to build itself (`go install` of `dlv` invokes cgo).

Notably absent: `COPY`, `go build`, `EXPOSE`, `CMD`. All of that made sense for a container meant to *run* a prebuilt debuggee headlessly — it's dead weight here. Dev Containers bind-mounts your whole repo live at container start, so anything an image-build-time `COPY` put in place gets shadowed anyway. The image's only job is to provide the toolchain; your actual code always comes from the live mount.

---

## `devcontainer.json`

```json
{
  "name": "go-lessons",
  "build": {
    "dockerfile": "Dockerfile",
    "context": "."
  },
  "runArgs": ["--cap-add=SYS_PTRACE", "--security-opt", "seccomp=unconfined"],
  "customizations": {
    "vscode": {
      "extensions": ["golang.go"],
      "settings": {
        "go.toolsManagement.autoUpdate": false
      }
    }
  },
  "postCreateCommand": "go install github.com/go-delve/delve/cmd/dlv@latest"
}
```

- **`runArgs`** — `SYS_PTRACE` + `seccomp:unconfined` are still required even though nothing is remote anymore: Delve uses `ptrace` to control the process it launches, and Docker's default seccomp profile blocks that. Without these two flags, debugging fails with `could not attach to pid …: operation not permitted`.
- **`customizations.vscode.extensions`** — installs the Go extension *inside* the container, where it can find a real `go` binary. Your host-level VS Code install never needs it.
- **`postCreateCommand`** — installs `dlv` after the container is created, pinned rather than left to the extension's own auto-install prompt.

Prefer whichever Go version the image ships to be ≥ whatever `go.work` (or the module's own `go.mod`) declares — `GOTOOLCHAIN=auto` (default since Go 1.21) will silently fetch a newer patch/minor toolchain if needed, but it's cleaner to keep the base image current.

---

## `go.work`

```
go 1.27

use ./code/test1
```

A [Go workspace file](https://go.dev/ref/mod#workspaces) lets several separate `go.mod` modules be developed and built together as one unit, without `replace` directives or publishing. With this file present, running `go build`/`go test`/gopls from the repo root resolves imports across every `use`'d module as if they were one project. Add a `use ./code/<new-module>` line for each new practice exercise; a module left out of `go.work` still builds fine standalone, it just won't get cross-module IntelliSense from the root.

`go.work` lives at the repo root, outside any single module — it's workspace-level config, not something any individual `go.mod` needs to know about.

---

## `launch.json`

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug (in devcontainer)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/code/test1"
    }
  ]
}
```

That's the entire config. No `host`, `port`, `substitutePath`, or `debugAdapter` override — those only exist to bridge a gap between two machines, and here there's only one. The Go extension passes `-gcflags=all=-N -l` automatically, so optimisations/inlining are disabled for you without managing that flag yourself.

Update `program` whenever you rename or add a practice module — it's a plain path, not derived from `go.work`, so it goes stale silently if the folder moves.

---

## What you get once attached

Everything you'd expect from a real debugger, plus one Go-specific win:

- **Breakpoints** — plus right-click for *conditional* breakpoints (`id == 42`), *hit count*, and *logpoints* (print without stopping, no recompile).
- **Step** over / into / out, and continue.
- **Variables** panel with structs expanded field by field; **Watch** for arbitrary expressions.
- **Call Stack** panel — lists **every live goroutine**, and clicking one switches the whole stack/variables view into it. That's how you see what a worker pool from [Lesson 17](../lessons/17-concurrency-sync-and-context.md) is actually doing.
- **Debug Console** — evaluates Go expressions in the current frame. Delve commands work with a `dlv` prefix, e.g. `dlv goroutines`, `dlv stack`.
- `runtime.Breakpoint()` in source stops the debugger at that line — handy for a spot you can't click, like generated code.
- Program stdout/stderr shows up directly in the Debug Console, since it's a local launch, not a remote attach.

---

## Gotchas

| Symptom | Cause | Fix |
|---|---|---|
| Container exits immediately after start/rebuild | Normal mid-reconnect churn (Dev Containers stops the old container before starting a new one) | Check `docker ps` a few seconds later; only worry if it never comes back `Up` |
| Editor/gopls shows stale file content after an external edit | Editor buffer or gopls cache, not the file itself (confirm with `docker exec <id> cat <file>`) | Close & reopen the tab, then **Go: Restart Language Server** |
| Renaming `"name"` in `devcontainer.json` "to force a rebuild" does nothing | `name` is a cosmetic label, not part of the image/container identity | **Dev Containers: Rebuild Container Without Cache** |
| Rebuild still looks stale | Docker layer cache | **Rebuild Container Without Cache**, or manually `docker rmi` the built image first |
| `dlv: command not found` after `postCreateCommand` | Missing `build-essential` — `go install` of `dlv` needs cgo | Install `build-essential` in the Dockerfile |
| `operation not permitted` on attach | Missing ptrace permission | `runArgs`: `--cap-add=SYS_PTRACE --security-opt seccomp=unconfined` |
| "Could not find statement at …" / variables show `<optimized out>` | Binary built optimised | Shouldn't happen via F5 (extension passes `-gcflags=all=-N -l` automatically) — check for a stray manual `go build` |
| F5 fails with a path/package error after renaming a module folder | `launch.json`'s `program` still points at the old path | Update `program` to match the module's current location |
| A module builds fine standalone but has no cross-module IntelliSense | Missing `use ./code/<module>` line in `go.work` | Add it, then **Go: Restart Language Server** |

---

## Recap

- The container image only needs to provide the toolchain (`golang:` base + `build-essential` for cgo) — `COPY`/`go build`/`CMD` are pointless once the live bind mount takes over.
- `runArgs` still needs `SYS_PTRACE` + `seccomp:unconfined` — Delve's `ptrace` requirement doesn't go away just because nothing is remote.
- `go.work` ties multiple `code/*` modules into one workspace for gopls, without `replace` directives.
- `launch.json` collapses to an ordinary local `launch`/`debug` config — no `host`, `port`, or `substitutePath`, because there's no second machine to bridge to anymore.

Next: **[HTTP Servers with `net/http`](02-http-server.md)** — something worth putting a breakpoint in.

[← Back to roadmap](../roadmap.md)
