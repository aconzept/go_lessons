# Lesson 3 — Data Types

> For TypeScript developers learning Go.
> Back to [roadmap](../roadmap.md).

This lesson covers:
- Numeric types (integers, floats, complex)
- Boolean
- Runes
- Strings (raw vs interpreted literals)
- Type conversion

In TypeScript you have one `number` and one `string`. Go gives you many sizes of integers and floats, separates "byte" from "character", and refuses to convert between numeric types for you. That sounds like more work, but in practice it pays off in performance, correctness at boundaries (databases, network protocols), and clarity.

---

## Numeric types

### Integers

| Go type | Range | TS equivalent |
|---|---|---|
| `int8`, `int16`, `int32`, `int64` | signed | `number` (but only up to 2^53 safely) |
| `uint8`, `uint16`, `uint32`, `uint64` | unsigned | — |
| `int` | platform-sized (32 or 64 bit) | — |
| `uint` | platform-sized unsigned | — |
| `byte` | alias for `uint8` | — |
| `rune` | alias for `int32` (a Unicode code point) | — |

```go
var age int = 30
var port uint16 = 8080
var bytesRead int64 = 1 << 40
```

**Rules of thumb:**
- Default to `int` for counters and indices.
- Use a specific size (`int32`, `uint64`, …) when interacting with binary protocols, file formats, or databases that demand it.
- Use `byte` when you mean "raw 8 bits"; use `rune` when you mean "a Unicode character".

### Floating point

```go
var pi float64 = 3.14159
var temp float32 = 36.6
```

Default to `float64` — it's what `1.5` is by default, and what most stdlib math functions accept. `float32` exists mainly for graphics and compact binary formats.

### Complex numbers

Go has built-in `complex64` and `complex128`. You almost certainly won't need them outside of scientific code — they exist, that's all you need to know for now.

```go
c := complex(2, 3)   // 2 + 3i
```

---

## Boolean

```go
var ok bool = true
done := false
```

Two important differences from TS:

1. **No truthiness.** `if x` where `x` is a string or number is a *compile error*. You must write `if x != ""` or `if x != 0`. This eliminates a whole category of bugs you've trained yourself to avoid in JS/TS.
2. **No `||` for defaults.** `name || "anonymous"` doesn't work because `||` returns a `bool`, not the operand. Use an `if`, the comma-ok idiom, or a small helper.

```go
// TypeScript
const display = name || "anonymous";

// Go
display := name
if display == "" {
    display = "anonymous"
}
```

---

## Runes

A `rune` is a single Unicode code point, stored as an `int32`. Rune literals use single quotes:

```go
r := 'A'           // rune = 65
heart := '❤'        // rune = 0x2764
```

This is one of the few places where Go's distinction between bytes and characters becomes visible. TS hides this — a JS string is a sequence of UTF-16 code units, and indexing into it sometimes does the wrong thing on emoji. Go is more honest:

```go
s := "héllo"
fmt.Println(len(s))           // 6  — bytes, not characters
fmt.Println(len([]rune(s)))   // 5  — actual characters
```

We come back to this below.

---

## Strings

A Go `string` is an **immutable sequence of bytes** (typically UTF-8 encoded). Strings are values, not references — comparing with `==` does a byte-wise comparison.

### Interpreted vs raw literals

```go
// Interpreted: double quotes. Escape sequences work.
s1 := "line1\nline2\t\"quoted\""

// Raw: backticks. No escapes, can span multiple lines.
s2 := `line1
line2 with literal \n and "quotes"`
```

Comparison:

| Style | TS analogue | When to use |
|---|---|---|
| `"..."` interpreted | `"..."` (with `\n`, etc.) | Most strings. |
| `` `...` `` raw | template literal *without* substitution | Multi-line strings, regex patterns, JSON/SQL snippets. |

Go has **no string interpolation**. The equivalent of `` `Hello, ${name}!` `` is:

```go
import "fmt"
greeting := fmt.Sprintf("Hello, %s!", name)
```

`fmt.Sprintf` (and friends) use `printf`-style verbs: `%s` (string), `%d` (int), `%v` (default format for anything), `%+v` (struct with field names), `%T` (the type itself, useful when debugging).

### Length and indexing

`len(s)` is the **byte length**, not the character count. Indexing `s[i]` returns a `byte`, not a `rune`.

```go
s := "héllo"      // 'é' is 2 bytes in UTF-8
len(s)            // 6
s[0]              // 'h' (104)
s[1]              // first byte of 'é', not 'é' itself
```

To iterate by code point, use `for range`, which decodes UTF-8 for you:

```go
for i, r := range "héllo" {
    fmt.Printf("%d: %c\n", i, r)
}
// 0: h
// 1: é     (note: i jumps to 3 next — byte offset)
// 3: l
// 4: l
// 5: o
```

For most app code where the string came from a known UTF-8 source (HTTP body, JSON, file), this just works. The byte/rune distinction matters when slicing user-entered text — use the `unicode/utf8` package or convert to `[]rune` first.

---

## Type conversion

Go does **no implicit numeric conversions**. This is unlike TS, where `1 + "2"` silently becomes `"12"`. In Go that's a compile error.

```go
var i int = 10
var f float64 = i       // ERROR
var f float64 = float64(i)   // OK — explicit conversion
```

This applies to integer sizes too:

```go
var a int32 = 5
var b int64 = a        // ERROR
var b int64 = int64(a) // OK
```

It's verbose, but you never get a silent truncation, sign change, or precision loss — every conversion is visible in the diff.

### String ↔ bytes ↔ runes

```go
s := "héllo"

b := []byte(s)        // bytes (UTF-8) — common for I/O
r := []rune(s)        // code points — when you need char-by-char

back1 := string(b)    // bytes  → string
back2 := string(r)    // runes  → string
```

`string(intValue)` is **not** the way to convert a number to its decimal string — that converts the int to the *rune* of that code point. Use `strconv`:

```go
import "strconv"

n := 42
s := strconv.Itoa(n)                 // "42"
n2, err := strconv.Atoi("42")        // 42, nil

f := 3.14
s2 := strconv.FormatFloat(f, 'f', 2, 64)   // "3.14"
```

`go vet` will flag the `string(n)` mistake — keep `vet` in your build.

---

## Recap

- Pick numeric types intentionally: `int` for general counts, sized types for protocols/storage, `float64` by default.
- No truthiness, no implicit conversions — both eliminate whole bug classes at the cost of more explicit code.
- A `string` is bytes, a `rune` is a code point; `len(s)` is bytes; `for range` over a string yields runes.
- For multi-line strings or regex/JSON/SQL, use backtick (raw) literals. For interpolation, use `fmt.Sprintf`.
- Convert with explicit `T(x)`; convert numbers to/from strings with `strconv`.

Next: **Commands & Docs** — the rest of the `go` toolchain and how Go's documentation system works.

[← Back to roadmap](../roadmap.md)
