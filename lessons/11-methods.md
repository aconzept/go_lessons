# Lesson 11 — Methods

> Back to [roadmap](../roadmap.md).

This lesson covers:
- Methods vs plain functions
- Value receivers vs pointer receivers
- How to choose between them
- Methods on non-struct types
- Method values and method expressions
- Method sets — and why they matter for interfaces

Go has no classes. Behaviour is attached to types by declaring **methods**: functions with an extra "receiver" before the name. That's the whole feature, but a few details are worth understanding properly because they show up again when we get to interfaces.

---

## Declaring a method

```go
type Rectangle struct {
    W, H float64
}

func (r Rectangle) Area() float64 {
    return r.W * r.H
}

r := Rectangle{W: 3, H: 4}
fmt.Println(r.Area())   // 12
```

The `(r Rectangle)` between `func` and the name is the **receiver**. Read it as: "this is a method on `Rectangle`, and `r` is the name we use for the value it was called on." `r` is just a parameter; you can call it anything (`self`, `x`, …) but convention is a one- or two-letter abbreviation of the type name.

You can only declare methods on **named types defined in the current package**. You cannot add a method to a type from another package (no monkey-patching). To extend an outside type, wrap it in your own:

```go
type LoudPrinter struct{ *log.Logger }

func (p LoudPrinter) Shout(s string) { p.Printf("!!! %s !!!", s) }
```

---

## Methods vs plain functions

These two are nearly equivalent:

```go
func Area(r Rectangle) float64       { return r.W * r.H }
func (r Rectangle) Area() float64    { return r.W * r.H }
```

When to prefer a method:

- The function naturally "belongs to" the type — most operations on `Rectangle`.
- You want to satisfy an interface (next lesson).
- Discoverability — `r.` will list the methods.

When to prefer a plain function:

- The operation involves two or more equally-important types and no one "owns" it.
- You're working with a type you don't own, or your code shouldn't tie itself to that type.

Don't agonise. The compiler treats them similarly; the choice is about what reads cleanly to a human.

---

## Value vs pointer receivers

The receiver can be the value type or a pointer to it:

```go
func (r Rectangle)  Area()   float64 { return r.W * r.H }   // value receiver
func (r *Rectangle) Scale(k float64) { r.W *= k; r.H *= k } // pointer receiver
```

The difference is the same one you saw with function parameters:

- **Value receiver**: the method gets a *copy* of the receiver. Any mutation is on the copy and is invisible to the caller.
- **Pointer receiver**: the method gets the *address*; mutations affect the original.

Calling looks identical in source — Go inserts the `&` or `*` for you when needed:

```go
r := Rectangle{W: 3, H: 4}
r.Scale(2)            // Go takes &r automatically
fmt.Println(r.Area()) // 24
```

The auto-addressing only works on **addressable** values — typically a variable. Calling a pointer-receiver method on a map-indexed value like `m["k"].Scale(2)` is a compile error, because `m["k"]` isn't addressable.

### How to choose

A short decision tree that holds up in practice:

1. **Does the method mutate the receiver?** → pointer.
2. **Is the receiver "big" (more than a couple of machine words, or contains a slice/map you read often)?** → pointer, to avoid copying.
3. **Does the type already have any pointer-receiver method?** → use pointer for *all* methods on it, so the method set is consistent (more on this below).
4. **Otherwise, default to pointer for structs.**
5. **For small immutable types** (a 2-D point, a duration, a string-wrapper) value receivers are fine and arguably nicer — they make the value feel "primitive".

The big rule: **be consistent within a type**. Don't mix value and pointer receivers without a reason; doing so causes the method-set gotcha below.

---

## Method sets (and why mixing is bad)

The **method set** of a type is the set of methods callable on a receiver of that type. The rule:

- Method set of `T` = all methods with receiver `T`.
- Method set of `*T` = all methods with receiver `T` **or** `*T`.

So a `*Rectangle` has both `Area` (value receiver) and `Scale` (pointer receiver). A `Rectangle` value has *only* `Area` — well, you can *call* `Scale` on a `Rectangle` variable because Go takes its address automatically, but the *method set of the type* doesn't include `Scale`.

Why does it matter? **Interface satisfaction is decided by the method set.** If an interface requires both `Area()` and `Scale()`, only `*Rectangle` satisfies it, not `Rectangle`. We'll see this in the next lesson — it explains why you'll often see code passing `&thing` to an interface parameter when `thing` looks like it should work.

---

## Methods on non-struct types

Methods aren't limited to structs. You can declare them on any **named type** you define:

```go
type UserID int

func (u UserID) String() string {
    return fmt.Sprintf("user-%d", u)
}

func (u UserID) IsAnon() bool { return u == 0 }
```

This is a powerful pattern — it gives a primitive a name *and* behaviour without the overhead of a wrapper struct. You can't add methods to the built-in `int`, but you can to your own `UserID`, even though `UserID` is "just an int" at runtime.

The same trick works on slices and maps:

```go
type Names []string
func (n Names) Contains(s string) bool { /* ... */ }
```

A worked-out version of this pattern is `time.Duration`, which is `type Duration int64` with methods like `Seconds()` and `String()`.

---

## Method values and method expressions

Two niche but worth-knowing-they-exist forms:

```go
r := Rectangle{W: 3, H: 4}

// Method value — bound to a specific receiver
f := r.Area
f()                      // 12

// Method expression — function with the receiver as the first argument
g := Rectangle.Area
g(r)                     // 12
```

The method-value form is occasionally useful when you want to hand a method to something that takes a `func() T`. The expression form is rarer — mostly a way to be explicit when reading reflection-heavy code.

---

## A small worked example

```go
type Counter struct{ n int }

func (c *Counter) Inc()         { c.n++ }
func (c *Counter) Add(delta int){ c.n += delta }
func (c  Counter) Value() int   { return c.n }   // pure read; value receiver is fine

func main() {
    c := &Counter{}      // construct via pointer because we have pointer methods
    c.Inc()
    c.Add(10)
    fmt.Println(c.Value())   // 11
}
```

Notice `Value()` is a value receiver despite the type being used through a pointer. That's allowed, but if you're being strict about consistency you can make every method pointer-receiver and not think about it. Both choices show up in real codebases.

---

## Recap

- A method is a function with a receiver: `func (r T) Name(...) ...`.
- Value receivers copy the receiver; pointer receivers don't. Use a pointer when you mutate, the type is large, or any other method on the type is pointer-receiver.
- Method sets decide interface satisfaction: `*T` includes value-receiver methods, `T` does not include pointer-receiver methods.
- Methods aren't only for structs — define them on any named type to give a primitive behaviour.
- Be consistent within a type; mixing receivers is the most common foot-gun.

Next: **Interfaces** — empty interface, embedding, type assertions, type switches.

[← Back to roadmap](../roadmap.md)
