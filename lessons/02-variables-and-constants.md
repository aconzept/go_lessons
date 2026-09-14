# Lesson 2 — Variables & Constants

> For TypeScript developers learning Go.
> Back to [roadmap](../roadmap.md).

This lesson covers:
- `var` vs `:=`
- Zero values
- `const` and `iota`
- Scope and shadowing

---

## `var` vs `:=`

Go has two ways to declare a variable. They look different but do the same thing — pick whichever reads better.

```go
// Long form: explicit type, optional value
var name string = "Ada"
var age  int    = 36
var ok   bool                // no value → zero value (false)

// Short form: type inferred from the right-hand side
name := "Ada"
age  := 36
ok   := false
```

| TypeScript | Go |
|---|---|
| `let name: string = "Ada"` | `var name string = "Ada"` |
| `let name = "Ada"` | `name := "Ada"` |
| `const name = "Ada"` *(reassignment forbidden)* | `const name = "Ada"` *(but with stricter rules — see below)* |

### Rules for `:=`

- **Only inside functions.** At the package level (top of a file, outside any function) you must use `var`.
- **At least one new variable on the left.** This is legal:
  ```go
  name, err := getName()      // both new
  name, err  = getName()      // both already exist → use =
  name, err := getName()      // err is new, name is reused → still OK
  ```
- Compiler **enforces "no unused locals"**. This won't compile:
  ```go
  func main() {
      x := 42   // declared and not used
  }
  ```
  This catches the same bugs ESLint's `no-unused-vars` catches in TS, but it's a hard error, not a warning.

### Multiple declaration

```go
var (
    host string = "localhost"
    port int    = 8080
    tls  bool                 // zero value: false
)
```

The grouped `var (...)` form is the Go equivalent of declaring a constants block in TS — pure convention, nothing magic about it.

---

## Zero values

There is **no `undefined`** in Go. Every type has a zero value, and an uninitialized variable gets that zero value automatically.

| Type | Zero value |
|---|---|
| numeric (`int`, `float64`, ...) | `0` |
| `bool` | `false` |
| `string` | `""` (empty, not nil) |
| pointer, slice, map, channel, function, interface | `nil` |
| struct | a struct whose fields are all zero-valued |

```go
var s string     // ""
var n int        // 0
var p *User      // nil
var u User       // User{} — every field zeroed
```

For a TS developer this is a small but important shift in mindset:

```ts
let n: number;   // undefined — runtime error if used before assignment
```
```go
var n int        // 0 — perfectly safe to use immediately
```

This means you write **far less** defensive code like `if (x === undefined)`. The trade-off: you have to distinguish between *"unset"* and *"explicit zero"* yourself when the difference matters (often using a pointer: `*int` is `nil` when unset, or a "sql.NullInt64"-style struct).

---

## `const`

`const` looks similar to TS but is meaningfully different:

```go
const Pi = 3.14159
const Greeting = "Hello"

const (
    StatusActive   = "active"
    StatusInactive = "inactive"
)
```

Differences from TS:

1. **Compile-time only.** A `const` value must be computable at compile time. So this is fine:
   ```go
   const MaxRetries = 3 * 2
   ```
   But this is not:
   ```go
   const Now = time.Now()   // ERROR: not a constant expression
   ```
   In TS, `const x = computeThing()` is allowed; in Go, use a `var` for that.

2. **Only basic types.** Numbers, strings, booleans, and runes. You can't have a `const` slice, map, or struct. (Use `var` and discipline, or freeze with helper accessors.)

3. **Untyped by default.** A `const` like `const N = 10` has no fixed type until used. It can flex into any compatible numeric type:
   ```go
   const N = 10
   var i int     = N    // OK
   var f float64 = N    // OK
   ```
   This is more flexible than `as const` in TS.

---

## `iota`

`iota` is a small DSL for incrementing constants inside a `const (...)` block. It is Go's answer to TS enums — simpler, but with sharp edges.

```go
const (
    Sunday = iota   // 0
    Monday          // 1
    Tuesday         // 2
    Wednesday       // 3
    Thursday        // 4
    Friday          // 5
    Saturday        // 6
)
```

Each line in the block re-uses the expression from the previous line; `iota` increments by one starting at 0 inside each `const` block.

Useful patterns:

```go
// Bit flags
const (
    FlagRead    = 1 << iota   // 1
    FlagWrite                 // 2
    FlagExecute               // 4
)

// Skip a value
const (
    _ = iota   // discard 0
    KB = 1 << (10 * iota)   // 1 << 10
    MB                       // 1 << 20
    GB                       // 1 << 30
)
```

For named string enums, `iota` doesn't help — write them out:

```go
type Status string

const (
    StatusActive   Status = "active"
    StatusInactive Status = "inactive"
)
```

This is the idiomatic Go equivalent of a TS string literal union (`type Status = "active" | "inactive"`). Note the **named type** (`type Status string`) — it gives you a bit of compile-time safety so you can't pass any old string where a `Status` is expected.

---

## Scope and shadowing

Scope rules are familiar from TS — block-scoped via `{}`. Two Go-specific things to watch out for:

### Package vs function scope

Anything declared outside a function is **package-scoped** (visible to every file in the same package). There is no separate "file scope".

```go
// file: config.go
package app
var defaultPort = 8080   // visible from every *.go file in package app
```

### Shadowing with `:=`

This is the most common Go gotcha for newcomers:

```go
func process() error {
    err := first()
    if err != nil { return err }

    if cond {
        err := second()        // ⚠️ NEW err, shadows the outer one
        log.Println(err)
    }
    // outer err is still whatever first() returned
    return err
}
```

Because `:=` declares a new variable, using it inside a nested block creates a *different* `err` that shadows the outer one. The fix:

```go
if cond {
    err = second()             // = instead of := reuses the outer err
    log.Println(err)
}
```

Most linters (`go vet -shadow`, `golangci-lint` with `govet.shadow` enabled) flag this. Coming from TS, where `let` in a nested block has the same behaviour, it's the *implicit* form of `:=` that bites — there's no `let` keyword to remind you a new binding is being created.

---

## Recap

- `var x T = v` is the long form; `x := v` is the short form (functions only).
- No `undefined` — every type has a zero value, and uninitialized variables are safe to use.
- `const` is compile-time only and limited to basic types; `iota` generates incrementing values inside a `const` block.
- Watch for accidental shadowing when using `:=` inside `if`/`for` blocks — enable the shadow linter.

Next: **Data Types** — numbers, booleans, runes, strings, and explicit conversion.

[← Back to roadmap](../roadmap.md)
