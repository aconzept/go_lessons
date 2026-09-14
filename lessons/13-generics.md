# Lesson 13 — Generics

> Back to [roadmap](../roadmap.md).

This lesson covers:
- Why generics exist in Go
- Generic functions: type parameters and inference
- Generic types
- Constraints — the `comparable` and `ordered` story
- Type sets and the `~T` underlying-type marker
- When to use generics — and when not to

Generics arrived in Go 1.18 (2022). They're a measured addition rather than a TypeScript-grade type system, deliberately chosen to keep the rest of the language simple. Most idiomatic Go is still non-generic; generics are best when you previously would have written the same loop or container twice for different element types.

---

## Why generics?

Before 1.18, you wrote a `Min` function once per type:

```go
func MinInt(a, b int) int       { if a < b { return a }; return b }
func MinFloat(a, b float64) float64 { ... }
```

Or you went via `any` and lost type safety:

```go
func Min(a, b any) any { ... }   // caller has to type-assert
```

Generics let you write it once, typed:

```go
func Min[T cmp.Ordered](a, b T) T {
    if a < b { return a }
    return b
}

Min(3, 4)           // int      → 3
Min(2.5, 1.5)       // float64  → 1.5
Min("a", "b")       // string   → "a"
```

---

## Anatomy of a generic function

```go
func Map[T, U any](in []T, f func(T) U) []U {
    out := make([]U, len(in))
    for i, v := range in {
        out[i] = f(v)
    }
    return out
}
```

Read it left to right:

- `[T, U any]` — **type parameters**, declared in square brackets after the function name. `T` and `U` are placeholders; `any` is the **constraint**, meaning "no constraint".
- The rest is a normal signature using `T` and `U` where you'd use a concrete type.

Call it without spelling out the type parameters when the compiler can infer them:

```go
nums := []int{1, 2, 3}
doubled := Map(nums, func(n int) int { return n * 2 })   // []int

words := []string{"go", "is", "ok"}
lens  := Map(words, func(s string) int { return len(s) })  // []int
```

When inference fails (the type parameters don't appear in any argument), pass them explicitly:

```go
zeros := Map[int, float64]([]int{1,2,3}, func(n int) float64 { return 0 })
```

---

## Generic types

You can also parameterise types:

```go
type Stack[T any] struct {
    data []T
}

func (s *Stack[T]) Push(v T)      { s.data = append(s.data, v) }
func (s *Stack[T]) Pop() (T, bool) {
    var zero T
    if len(s.data) == 0 {
        return zero, false
    }
    last := s.data[len(s.data)-1]
    s.data = s.data[:len(s.data)-1]
    return last, true
}

s := Stack[int]{}
s.Push(1); s.Push(2); s.Push(3)
v, _ := s.Pop()   // 3
```

The `var zero T` trick is the standard way to produce "the zero value of T" inside a generic body — since you don't know the concrete type, you can't write `0` or `""`.

A few rules to know:
- The type parameter list lives on the type declaration, not on each method. Each method then writes `(s *Stack[T])` to bind `T`.
- Methods **cannot have their own type parameters**. If you need that, write a package-level generic function.
- You can't have generic interface methods either; the same rule applies.

---

## Constraints

A constraint says "T must be one of these types" or "T must have these methods". Constraints are themselves interfaces, but with one new power: they can list a set of allowed underlying types.

### `any` and `comparable`

The two pre-declared constraints:

- `any` — any type at all.
- `comparable` — types you can use with `==` and `!=`. Required if your generic code does `a == b`.

```go
func Index[T comparable](xs []T, target T) int {
    for i, v := range xs {
        if v == target { return i }
    }
    return -1
}
```

You cannot pass slices or maps where `comparable` is required (since `==` isn't defined for them) — the compiler will tell you.

### `cmp.Ordered` (Go 1.21+)

For things you can `<`/`>`/`<=`/`>=`:

```go
import "cmp"

func Max[T cmp.Ordered](a, b T) T {
    if a > b { return a }
    return b
}
```

`cmp.Ordered` is the union of all signed/unsigned integers, floats, and strings — i.e., everything `<` works on. Before 1.21 you had to declare this constraint yourself; you'll still see hand-rolled `Ordered` in older code.

### Custom constraints

A constraint is just an interface that may list types:

```go
type Number interface {
    int | int64 | float32 | float64
}

func Sum[T Number](xs []T) T {
    var total T
    for _, x := range xs { total += x }
    return total
}
```

The `|` is a **type set** — "any of these". You can mix type sets with method requirements:

```go
type StringerNumber interface {
    Number
    String() string
}
```

### The `~` operator

`~int` means "any type whose underlying type is int", which lets your constraint accept derived types:

```go
type Ordered interface {
    ~int | ~int64 | ~float32 | ~float64 | ~string
}

type UserID int
Max(UserID(1), UserID(2))   // works because UserID's underlying type is int
```

Without the `~`, only the literal `int` would be accepted. As a rule, the constraints in the standard library use `~` for primitive types — you should too.

---

## Inference, in more detail

Inference flows from arguments to type parameters. It works when:

1. A type parameter appears in an argument's type and the compiler can match it.
2. The match is consistent across all arguments.

Inference does **not** look at the return type to infer a parameter that only appears in the return:

```go
func Zero[T any]() T {
    var z T
    return z
}

x := Zero[int]()   // must specify; Zero() alone can't infer T
```

This shows up most often with generic constructors. When you find yourself writing `New[Thing]()`, that's why.

---

## When generics, when not

Generics are a great fit for:

- **Containers and algorithms over element type:** stacks, queues, sets, sorted maps, `Map`/`Filter`/`Reduce`, generic `Min`/`Max`/`Sum`/`Sort`.
- **Helpers that would otherwise return `any`** and force callers to assert.
- **Numeric code over multiple numeric types.**

They are *not* a replacement for interfaces. Use an interface when:

- The behaviour is the abstraction (`io.Reader`, `error`). Different concrete types do different things behind the same method set — that's polymorphism.

Use a type parameter when:

- The *type* is the abstraction. The code is the same for every T; T just plugs in.

The standard library now ships generic packages worth knowing — these are the cases you should reach for them, rather than rolling your own:

- `slices` — `slices.Sort`, `Contains`, `Index`, `Reverse`, `Clone`, `Concat`, `BinarySearch`.
- `maps`   — `maps.Keys`, `Values`, `Clone`, `Equal`.
- `cmp`    — `cmp.Compare`, `cmp.Less`, `cmp.Ordered`.
- `sync`   — `sync.OnceValue[T]`, `sync.OnceValues[T1, T2]`.

A small cautionary note: generics can make code harder to read when they replace something already clear. A two-line `for` loop is often better than a generic helper. If you find yourself constraining `T` with three different interfaces just to make a function work for one extra type, take a breath — a plain function or interface might be the right answer.

---

## Recap

- Generic functions: `func F[T C](...)` where `C` is a constraint; types usually inferred from arguments.
- Generic types: `type X[T any] struct{...}`; methods bind `T` via `(x X[T])`, no per-method type parameters.
- `any`, `comparable`, and `cmp.Ordered` are the constraints you'll reach for most. Custom constraints use type sets (`|`) and `~T` to accept derived types.
- The standard library's `slices`, `maps`, and `cmp` packages cover most "generic algorithm" needs.
- Generics complement interfaces, not replace them. Use interfaces for behaviour, generics for code-once-over-many-types.

Next: **Code Organization** — modules, packages, and import rules.

[← Back to roadmap](../roadmap.md)
