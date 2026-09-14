# Lesson 12 — Interfaces

> Back to [roadmap](../roadmap.md).

This lesson covers:
- Interface basics — declaration and satisfaction
- Implicit (structural) satisfaction
- The empty interface, `any`
- Interface values: dynamic type + dynamic value, and `nil`
- Type assertions and the comma-ok form
- Type switches
- Embedding interfaces
- Where interfaces belong and how to size them

Interfaces are the most distinctive feature of Go's type system. They're how polymorphism, dependency injection, and most stdlib extension points work. The headline: **satisfaction is structural and implicit** — there is no `implements` keyword.

---

## Declaring an interface

```go
type Stringer interface {
    String() string
}
```

A type "satisfies" `Stringer` if it has a method with that exact signature. You don't declare the relationship anywhere:

```go
type UserID int

func (u UserID) String() string {
    return fmt.Sprintf("user-%d", u)
}

// UserID automatically satisfies Stringer.
var s Stringer = UserID(42)
fmt.Println(s.String())   // user-42
```

This is similar to TypeScript's structural typing of object shapes, but for *methods* instead of *fields*. The big practical consequence: you can define a new interface in your own package and have stdlib types satisfy it without modifying them. The interface lives near the consumer, not the implementer.

A famous example from the standard library — these are the entire definitions:

```go
type Reader interface { Read(p []byte) (n int, err error) }
type Writer interface { Write(p []byte) (n int, err error) }
type Closer interface { Close() error }
```

Tiny interfaces are idiomatic. `*os.File`, `*bytes.Buffer`, `net.Conn`, `*gzip.Reader`, and many others happen to satisfy them, which is why functions like `io.Copy(dst Writer, src Reader)` can wire arbitrary sources and sinks together.

---

## Interface values: type + value

An interface value internally holds two things: a **dynamic type** and a **dynamic value**.

```go
var w io.Writer        // dynamic type = nil, dynamic value = nil → interface is nil
w = os.Stdout          // dynamic type = *os.File, dynamic value = the file handle
```

You can think of it as a two-word `(type, value)` pair. This matters in one place: **nil checks**.

```go
var f *os.File = nil
var w io.Writer = f    // w is NOT nil — dynamic type is *os.File, value is nil
fmt.Println(w == nil)  // false
```

This is the classic "typed nil" trap. The interface contains a non-nil type descriptor even though the underlying pointer is nil, so `w == nil` is false. If you've ever seen a Go function appear to return `nil` for an error and yet `if err != nil` fires anyway — this is why. Avoid returning a *typed* nil; return an untyped `nil` from a function whose return type is the interface:

```go
// bad — typed nil leaks out as a non-nil interface
func find() error {
    var e *MyError = nil
    return e
}

// good
func find() error {
    return nil
}
```

---

## `any` (the empty interface)

The empty interface `interface{}` is satisfied by every type — it requires no methods. In Go 1.18+ there's an alias for it called `any`:

```go
var x any = 42
x = "hello"
x = []int{1, 2, 3}
```

You'll see `any` in:

- `fmt.Println(...any)` and the rest of the `fmt` family.
- `encoding/json`'s `Unmarshal` into a `map[string]any` for unknown shapes.
- Generic plumbing before Go 1.18.

Reach for `any` when you genuinely cannot constrain the type. For "this is a number" or "this has a method", use generics or a small interface — they keep type safety.

---

## Type assertions

A type assertion extracts the concrete type out of an interface value:

```go
var w io.Writer = os.Stdout

f := w.(*os.File)        // panics if w's dynamic type isn't *os.File
fmt.Println(f.Name())

// Safe form — comma-ok (you've seen this before)
f, ok := w.(*os.File)
if !ok {
    // w wasn't a *os.File
}
```

The two-value form never panics. Same syntactic pattern as the map read and the channel receive.

Assertions also work for asking "does this satisfy another interface?":

```go
if c, ok := w.(io.Closer); ok {
    defer c.Close()
}
```

Useful when a function accepts `io.Reader` but optionally closes if the underlying reader supports it. This is a common stdlib idiom — `io.WriteString`, `io.Copy`, and friends all use it.

---

## Type switches

When you might be one of several types, the type switch is cleaner than a chain of assertions:

```go
func describe(x any) string {
    switch v := x.(type) {
    case int:
        return fmt.Sprintf("int %d", v)
    case string:
        return fmt.Sprintf("string %q", v)
    case fmt.Stringer:
        return v.String()
    case nil:
        return "nil"
    default:
        return fmt.Sprintf("unknown %T", x)
    }
}
```

Inside each case, `v` has the type of the case label. A case with multiple types (`case int, int64:`) gives `v` the interface type — the union semantics don't extend that far.

---

## Embedding interfaces

Like structs, interfaces can embed other interfaces. The composite requires every embedded method:

```go
type ReadCloser interface {
    Reader
    Closer
}
```

That's the actual `io.ReadCloser`. Embedding is just composition: it has the methods of `Reader` plus `Close() error`. No new method has to be added in `ReadCloser` itself.

You can also list methods directly in an embedded interface, but the pure-embedding form reads better when both halves are already named.

---

## Where to put interfaces

A piece of Go-flavoured advice that's surprisingly load-bearing:

> **Define interfaces in the package that *consumes* them, not the package that *implements* them.**

If your billing service needs to fetch a user, declare a small `UserFetcher` interface inside the billing package describing exactly the one method it needs. The user package keeps shipping its concrete `*UserStore` and knows nothing about your interface. That makes the user package easy to use without imports tangling, and lets you mock the dependency in tests by defining a one-method fake.

The opposite — a big `Service` interface in the same package as the only implementation — is a common newcomer instinct (and a habit from Java/C# DI containers). It tends to grow into a 30-method monolith and become friction rather than abstraction.

**Keep interfaces small.** "The bigger the interface, the weaker the abstraction" is a frequently-quoted Pike-ism. Most stdlib interfaces have one or two methods. Yours should too. If a function only needs `Read`, take an `io.Reader`, not a `*os.File`.

---

## Quick reference

```go
// Declare
type Greeter interface {
    Greet(name string) string
}

// Implement (no keyword needed)
type Friendly struct{}
func (f Friendly) Greet(name string) string { return "hi, " + name }

// Use
func say(g Greeter, who string) { fmt.Println(g.Greet(who)) }
say(Friendly{}, "Ada")

// Probe
if h, ok := g.(interface{ Hands() int }); ok { /* probably a Human */ }

// Branch
switch v := g.(type) {
case Friendly: ...
case nil:      ...
default:       ...
}
```

---

## Recap

- Interfaces are method sets; satisfaction is structural and implicit — no `implements`.
- Interface values are `(dynamic type, dynamic value)` pairs. A typed nil is not a nil interface.
- `any` is the empty interface (Go 1.18+ alias). Use it only when you truly can't constrain.
- `x.(T)` asserts type; the comma-ok form is the safe one. `switch x.(type)` for multi-branch.
- Embed interfaces to compose them. Keep interfaces small and define them at the call site.

Next: **Generics** — type parameters, constraints, and inference.

[← Back to roadmap](../roadmap.md)
