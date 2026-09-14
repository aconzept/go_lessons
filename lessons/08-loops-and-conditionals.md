# Lesson 8 — Loops & Conditionals

> Back to [roadmap](../roadmap.md).

This lesson covers:
- `if` and `if-else` (and the init form)
- `switch` — including type switch
- The one and only loop: `for`
- `for range` over slices, maps, strings, channels, integers
- `break`, `continue`, labels, `goto`

Go has exactly one loop keyword (`for`) and a switch that does more than C's. There is no ternary, no `while`, no `do-while`, no `forEach` method on slices. The grammar is small on purpose.

---

## `if` / `if-else`

Mostly unsurprising — but **no parentheses around the condition** and **braces are required**:

```go
if n > 0 {
    fmt.Println("positive")
} else if n < 0 {
    fmt.Println("negative")
} else {
    fmt.Println("zero")
}
```

Single-statement `if` without braces is a syntax error. This rules out the dangling-else class of bug entirely.

### Init form

`if` can take an **init statement** before the condition, scoped to the `if` (and any `else`):

```go
if user, err := db.Find(id); err != nil {
    return err
} else {
    fmt.Println(user.Name)
}
```

`user` and `err` exist only inside the `if`/`else` blocks — they don't leak out. This is the idiomatic place to scope short-lived variables like `err`.

You'll most often see this with errors:

```go
if err := doThing(); err != nil {
    return err
}
```

### There is no ternary

```go
// not legal:    cond ? a : b
```

Write an `if`, or use a small helper. The deliberate verbosity is supposed to make conditional choices stand out in diffs.

---

## `switch`

`switch` in Go is broader than in C/JS. Three things to know upfront:

1. **No fallthrough by default.** Cases break automatically. Use the `fallthrough` keyword if you actually want it (rare).
2. **Cases can be any expression** of the right type, not just constants.
3. **A bare `switch` (no expression) is a clean `if-else-if` chain.**

```go
switch role {
case "admin", "owner":
    grantAll()
case "member":
    grantSome()
default:
    deny()
}
```

Multiple values per case with commas — replaces the `case A: case B: ...` stack from JS.

### Tag-less switch

```go
switch {
case n < 0:
    return "negative"
case n == 0:
    return "zero"
case n < 10:
    return "small"
default:
    return "large"
}
```

When you're reading Go code and see `switch` with no expression, it's just an `if/else if` chain written more vertically. Idiomatic when you have more than two or three branches.

### Init form

Like `if`, `switch` takes an init statement:

```go
switch x := compute(); {
case x < 0: ...
case x > 0: ...
}
```

### Type switch

A switch that branches on the **dynamic type** of an interface value. We'll cover interfaces properly later; for now, this is the syntax:

```go
switch v := x.(type) {
case int:
    fmt.Println("int:", v)
case string:
    fmt.Println("string:", v)
case nil:
    fmt.Println("nil")
default:
    fmt.Printf("unknown: %T\n", v)
}
```

`v` is typed as the case type inside each branch. This is one of the few places where Go does anything like narrowing.

---

## `for` — the only loop

Go folds C's `for`, `while`, and `do-while` into one keyword.

### Classic three-part form

```go
for i := 0; i < 10; i++ {
    fmt.Println(i)
}
```

### While-style — just leave the init and post out

```go
for n > 0 {
    n /= 2
}
```

### Infinite — no condition at all

```go
for {
    if done() { break }
    work()
}
```

This is how you write an event loop or a worker. `for {}` reads as "loop forever" — pair with `break`, `return`, or a channel receive to exit.

---

## `for range`

`range` produces one or two values depending on what you iterate over:

| You iterate over | First value | Second value |
|---|---|---|
| slice / array | index | element |
| map | key | value |
| string | byte index | rune (decoded UTF-8) |
| channel | received value | — *(only one value, loop ends when channel closes)* |
| integer `n` *(Go 1.22+)* | `0..n-1` | — |

```go
for i, v := range nums { ... }
for k, v := range users { ... }
for i, r := range "héllo" { ... }   // r is a rune; i is the byte offset
for v := range ch { ... }           // exits when ch is closed
for i := range 10 { ... }           // i = 0,1,...,9   (Go 1.22+)
```

Drop the value you don't need:

```go
for _, v := range nums { ... }   // value only
for i    := range nums { ... }   // index only
```

### Iterating maps — order is random

Worth saying again: map iteration order is unspecified and randomized per run. If you need stable output, collect keys, sort, then iterate.

### The pre-1.22 loop-variable trap

In Go versions before 1.22, the loop variable was **reused** across iterations. Capturing its address (or a closure over it) in a goroutine captured the same variable every time:

```go
// Pre-1.22 bug
for _, v := range items {
    go func() { use(v) }()   // every goroutine saw the last v
}
```

The fix was to shadow it: `v := v` inside the loop body, or pass `v` as a parameter to the goroutine.

From Go 1.22 onward, **each iteration gets a fresh variable**. The shadow trick is no longer necessary on modern toolchains. If you maintain older code, that's what those `x := x` lines were for and they can be removed when you bump the module's `go` directive to 1.22+.

---

## `break`, `continue`, labels

`break` exits the innermost `for` or `switch`. `continue` jumps to the next iteration of the innermost `for`.

**Labels** let you target an outer loop:

```go
Outer:
for _, row := range grid {
    for _, cell := range row {
        if cell == "X" {
            break Outer       // exits both loops
        }
    }
}
```

Labels are written `Name:` immediately before the `for` (or `switch`/`select`). They're sparingly used — but when you need them, they're cleaner than a sentinel flag.

`continue Label` works the same way for the inner-to-outer "skip to next iteration of the outer loop" case.

---

## `goto`

`goto Label` exists. Don't use it. The standard library uses it in a couple of places for hot-path optimisations and that's about it. Mentioned here only so you recognise it if you see it.

---

## Putting it together

A common shape — "loop, do work, error out cleanly":

```go
for _, item := range items {
    if err := process(item); err != nil {
        return fmt.Errorf("process %q: %w", item.ID, err)
    }
}
```

Note the `if err := ...; err != nil` pattern again — that init form keeps `err` scoped to the `if`. You'll write this thousands of times. It's worth letting your editor's snippet for `iferr` save you some keystrokes.

---

## Recap

- `if` has an optional init statement; braces required; no ternary.
- `switch` cases don't fall through by default; cases can be expressions; bare `switch` replaces long `if/else if` chains; type switches branch on interface dynamic type.
- `for` is the only loop; three forms (`init; cond; post`, `cond`, infinite) and `for range`.
- `range` over slice/array/map/string/channel/int; map order is randomized; on Go 1.22+ the loop variable is per-iteration.
- Labels for breaking/continuing outer loops; `goto` exists but you won't write it.

Next: **Functions** — multiple returns, variadics, closures, named returns.

[← Back to roadmap](../roadmap.md)
