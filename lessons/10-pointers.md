# Lesson 10 — Pointers

> Back to [roadmap](../roadmap.md).

This lesson covers:
- What a pointer is and the `&` / `*` operators
- Pointers to structs
- Pointers with maps and slices — and why you usually don't need them
- `nil` pointers and safe checks
- A quick mental model for the garbage collector
- Stack vs heap and escape analysis (overview)

Pointers in Go are the same idea as in C — a value that holds the memory address of another value — but **much** safer: no pointer arithmetic, no manual `free`, and the runtime + compiler prevent the classic dangling-pointer bugs.

If you've spent your career in JS/TS, the news here is mostly: pointers exist, they're tame, and you'll use them for two practical reasons — mutation and avoiding copies.

---

## The basics

```go
x := 10
p := &x          // p is a *int — pointer to x
fmt.Println(*p)  // 10        — dereference

*p = 99
fmt.Println(x)   // 99        — x was mutated through p
```

- `&x` → "address of x". Type: `*T` if `x` is `T`.
- `*p` → "value at p". Type: `T` if `p` is `*T`.
- The **zero value** of any pointer type is `nil`. Dereferencing `nil` panics.

There's no pointer arithmetic. You can't do `p++` or `p + 1`. (If you really need that, the `unsafe` package exists — and you almost certainly shouldn't.)

---

## Why pointers, in practice

There are really only two reasons to reach for a pointer:

### 1. Mutation

Pass-by-value means a function can't modify its caller's variable unless you pass a pointer:

```go
func inc(n int)  { n++ }
func incP(n *int) { *n++ }

x := 0
inc(x);   fmt.Println(x)   // 0
incP(&x); fmt.Println(x)   // 1
```

### 2. Avoiding copies of big structs

Passing or returning a large struct by value copies every field. For a `User` with a dozen fields, that's measurable. Passing `*User` copies only the pointer (8 bytes on 64-bit). Most domain types in Go codebases get passed around as pointers for this reason, even when mutation isn't the goal.

```go
func (s *Server) handle(req *Request) (*Response, error) { ... }
```

A handy rule: if a type has methods with pointer receivers (next lesson), use a pointer to it consistently throughout the codebase. Mixing pointer and value uses of the same type leads to subtle bugs.

---

## Pointers to structs

```go
type User struct {
    ID   int
    Name string
}

u := User{ID: 1, Name: "Ada"}
p := &u

// Field access — auto-dereference
fmt.Println(p.Name)        // "Ada"
p.Name = "Lovelace"        // sets u.Name
```

Notice you write `p.Name`, not `(*p).Name`. Go inserts the dereference for you on field access and method calls. This makes pointer-heavy code read almost the same as value code.

Constructing a `*T` literal in one step:

```go
u := &User{ID: 1, Name: "Ada"}   // u is *User
```

This is the most common form you'll see. The struct literal can't be the receiver of `&` directly in old Go versions, but for years now `&T{...}` has been the idiomatic way.

---

## Pointers with maps and slices

Maps, slices, and channels are already "reference-ish" — their headers point at heap-allocated storage. You don't usually need a pointer to one to mutate it through a function:

```go
func add(s []int, x int) []int {
    return append(s, x)              // append may reallocate — return the new header
}

func register(m map[string]int, k string) {
    m[k] = 1                         // visible to caller, no pointer needed
}
```

**Slices are the special case.** Because `append` may reallocate, the caller's slice header has to be updated by reassignment. You either:

1. Return the new slice (idiomatic): `s = add(s, 7)`.
2. Take a `*[]int` and reassign `*p = append(*p, 7)` (rare, ugly).

For maps, the function gets a copy of the small header but it points at the same hash table — direct mutation in the callee is visible to the caller.

So the rule of thumb: **don't write `*[]T` or `*map[K]V` parameters** unless you have a specific reason. They look like a bug.

---

## `nil` pointers

`nil` is the zero value of every pointer type:

```go
var p *User      // p == nil
if p == nil {
    p = &User{}
}
```

Dereferencing nil panics:

```go
var p *User
fmt.Println(p.Name)   // panic: runtime error: invalid memory address or nil pointer dereference
```

So check before deref when the pointer might be nil — typically right after a constructor or lookup returns it.

**Returning `*T` is also Go's "optional" type.** When something might not exist, a nullable pointer is a common pattern:

```go
func find(id int) *User {
    // ... return &user or nil
}

u := find(42)
if u == nil {
    // not found
    return
}
use(u)
```

For values where the *zero value* is a legitimate state (e.g. `0` is a real count), a pointer is the only way to say "this is unset". The other approach is to return `(T, bool)` or `(T, error)` — pick whichever the caller actually needs to act on.

---

## Memory: a quick mental model

You don't manage memory directly in Go. Two things happen automatically:

### Garbage collection

When nothing reachable references a value any more, the runtime reclaims it. You never call `free`. The collector is concurrent (it runs alongside your program) and tuned to keep pauses sub-millisecond for typical workloads.

This means **returning a pointer to a local variable is safe**:

```go
func newUser(name string) *User {
    u := User{Name: name}
    return &u              // perfectly fine
}
```

In C this would be undefined behaviour. In Go the compiler notices that `u`'s address escapes the function and allocates `u` on the heap instead of the stack so its lifetime can outlast the call. Same source code, different storage class — decided by the compiler.

### Stack vs heap, briefly

By default, local variables live on the function's stack frame and vanish when the function returns. The compiler's **escape analysis** asks: "could this variable be reachable after the function returns?" If yes, it moves the variable to the heap. If no, it stays on the stack.

You can ask the compiler what it decided:

```bash
go build -gcflags="-m" ./...
# ... main.go:5:6: moved to heap: u
```

For now, you don't need to think about this much. Two practical takeaways:

- Don't avoid `&local` thinking it'll dangle — it won't.
- Don't go out of your way to "force stack allocation" early. Write the natural code, then profile if it's hot. Premature escape-analysis tuning is wasted effort.

We'll return to this in the Advanced Topics section of the roadmap.

---

## Recap

- `&x` takes an address, `*p` dereferences it. No pointer arithmetic.
- Use pointers for **mutation** and to **avoid copying big structs**. Don't use them with maps and slices, which are already reference-ish.
- `nil` is the pointer zero value; deref panics. Pointers double as Go's "optional".
- The GC reclaims unreachable memory. Returning `&local` is safe — escape analysis promotes it to the heap automatically.
- `unsafe` exists; you almost never want it.

Next: **Methods, Interfaces, and Generics**.

[← Back to roadmap](../roadmap.md)
