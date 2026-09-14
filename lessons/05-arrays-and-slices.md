# Lesson 5 — Arrays & Slices

> Back to [roadmap](../roadmap.md).

This lesson covers:
- Arrays — fixed-size, value semantics
- Slices — the workhorse you'll use 95% of the time
- `len`, `cap`, `make`, and how slices grow
- Conversion between arrays and slices
- The aliasing gotchas

In Go, "list of things" is a slice. Arrays exist, but they're a low-level primitive most code doesn't reach for directly. Understanding both is necessary because slices are built on arrays.

---

## Arrays

An array has a **fixed length that is part of its type**.

```go
var a [3]int            // [0 0 0]
b := [3]int{10, 20, 30}
c := [...]int{1, 2, 3}  // length inferred → still [3]int
```

`[3]int` and `[4]int` are different types — they cannot be assigned to the same variable, and a function declared to accept `[3]int` won't accept a `[4]int`. This is the main reason arrays show up rarely in app code.

Arrays are **values**, not references. Assigning or passing one copies every element:

```go
a := [3]int{1, 2, 3}
b := a          // full copy
b[0] = 99
fmt.Println(a)  // [1 2 3]   — unaffected
fmt.Println(b)  // [99 2 3]
```

When you do use arrays, it's typically for:
- Cryptographic outputs with a known size (`[32]byte` for a SHA-256 digest).
- Performance-sensitive code where you want stack allocation and no indirection.

For everything else, reach for slices.

---

## Slices

A slice is a **view into an underlying array**. It has three pieces:

1. A pointer to some element of the backing array.
2. A length (`len`) — how many elements are visible.
3. A capacity (`cap`) — how many elements there are from the start of the view to the end of the backing array.

You almost never construct a slice header by hand. You write:

```go
nums := []int{1, 2, 3, 4, 5}   // literal — Go allocates the backing array
fmt.Println(len(nums), cap(nums))   // 5 5
```

Notice `[]int` with no length — that's what makes it a slice rather than an array. This is the type you'll see everywhere.

### Indexing and slicing

```go
nums := []int{10, 20, 30, 40, 50}

nums[0]      // 10
nums[1:4]    // [20 30 40]   — half-open: includes start, excludes end
nums[:2]     // [10 20]
nums[3:]     // [40 50]
nums[:]      // the whole slice
```

The result of a slicing expression is **another slice header pointing at the same backing array** — no copy. This is the source of both slices' speed and their most common bug.

### `append`

`append` adds elements. It may or may not allocate a new backing array:

```go
s := []int{1, 2, 3}
s = append(s, 4)        // returns a new slice header — reassign
s = append(s, 5, 6, 7)  // variadic
s = append(s, more...)  // spread another slice
```

**Always assign the result back.** `append(s, x)` without the assignment is a bug — the modified header is thrown away.

How it grows:
- If `cap(s) > len(s)`, `append` writes into the existing backing array and bumps `len`.
- Otherwise it allocates a new, bigger backing array, copies the elements, and returns a slice pointing at the new one.

The growth strategy is "double until you're past a threshold, then ~1.25x". You should not rely on the exact factor — only on the amortized cost being O(1).

### `make`

`make` lets you pre-size a slice when you know how many elements you'll add:

```go
buf := make([]byte, 1024)        // len = 1024, cap = 1024, zeroed
out := make([]int,  0, 100)      // len = 0,    cap = 100   — empty, room for 100
```

The `(len, cap)` form is the one to remember — when you're about to `append` in a loop, pre-sizing with the right `cap` avoids reallocations:

```go
items := make([]Item, 0, len(ids))
for _, id := range ids {
    items = append(items, fetch(id))
}
```

### `len` vs `cap` in pictures

```
backing array:  [ _ _ _ _ _ _ _ _ ]      cap = 8
slice s:        [ a b c ]                len = 3, cap = 8 (from a to end)
slice s[1:2]:     [ b ]                  len = 1, cap = 7 (from b to end)
```

`cap` of a sub-slice extends to the end of the *backing* array, not just the parent slice's window. This is the foundation of the aliasing traps below.

---

## Aliasing — the part that bites

Because a slice is a view, two slices can share storage. Mutating one is visible through the other.

```go
a := []int{1, 2, 3, 4, 5}
b := a[1:3]      // b = [2 3], shares storage with a

b[0] = 99
fmt.Println(a)   // [1 99 3 4 5]   ← a was modified
```

The classic surprise is with `append`:

```go
a := []int{1, 2, 3, 4, 5}
b := a[:2]                // b = [1 2], cap = 5 (!) — same backing array
b = append(b, 99)         // fits in cap → writes into a's storage
fmt.Println(a)            // [1 2 99 4 5]   ← element 3 of a got clobbered
```

If you want a copy that can't disturb its parent, **copy explicitly** or use the three-index slice form to clamp capacity:

```go
// 1. Copy
c := make([]int, len(a))
copy(c, a)

// 2. Slice + extra cap arg: a[low:high:max] — limits cap to (max - low)
b := a[:2:2]              // cap = 2 now; append must reallocate
```

The three-index form is uncommon in app code but invaluable when returning a slice you don't want callers to extend into your private buffer.

### `nil` vs empty slice

```go
var a []int        // nil  — len 0, cap 0, pointer nil
b := []int{}       // empty — len 0, cap 0, pointer non-nil (allocates a header)
```

Functionally these behave the same for `len`, `range`, and `append`. The only place the difference matters is JSON encoding: `nil` slices marshal to `null`, empty slices marshal to `[]`. Pick one for your API and stick to it.

---

## Conversion: array ↔ slice

### Array → slice

Slicing an array gives you a slice over it:

```go
arr := [5]int{1, 2, 3, 4, 5}
s := arr[:]              // slice over the full array
```

`s` now aliases `arr` — mutations through `s` are visible in `arr`. (If you don't want that, `copy` into a fresh slice.)

### Slice → array

Two forms, both added in modern Go:

```go
s := []int{1, 2, 3, 4, 5}

// Pointer to array — requires len(s) >= 4, otherwise panic
p := (*[4]int)(s)        // *[4]int aliasing s's storage

// Value array — copies. Requires len(s) >= 4.
var arr [4]int = [4]int(s)
```

These are useful when interoperating with APIs that demand a fixed-size array (crypto, low-level binary protocols).

---

## Iterating

`for range` over a slice yields `(index, value)`:

```go
for i, v := range nums {
    fmt.Println(i, v)
}

// index only
for i := range nums { ... }

// value only
for _, v := range nums { ... }
```

A subtle point: in Go versions before 1.22, the `v` was a *reused variable* — taking its address in a goroutine would capture the same pointer every iteration. From Go 1.22 onward, each iteration gets a fresh `v` and `i`. If you're on a recent toolchain you can stop worrying about this; if you maintain older code, look for `v := v` shadow copies inside loop bodies — that's the pre-1.22 fix and you can now delete it.

---

## Common patterns

### Filter

```go
out := items[:0]                 // reuse storage
for _, x := range items {
    if keep(x) {
        out = append(out, x)
    }
}
items = out
```

Using `items[:0]` as the destination is an idiom — it filters in place without allocating, as long as you don't read the original `items` afterwards.

### Delete index `i`

```go
s = append(s[:i], s[i+1:]...)
```

Or in Go 1.21+:

```go
import "slices"
s = slices.Delete(s, i, i+1)
```

The standard library's `slices` package (1.21+) has `Sort`, `Contains`, `Index`, `Reverse`, `Clone`, `Concat`, and more. Prefer it over hand-rolled loops when one of those fits.

### Iterating safely while modifying

Don't. Build a new slice, or collect indices to delete and apply them in a second pass.

---

## Recap

- Array `[N]T`: fixed length, value semantics, rarely used directly.
- Slice `[]T`: a `(ptr, len, cap)` view over a backing array. The default "list" type.
- `append` may reallocate; **always reassign**. `make([]T, 0, n)` to pre-size.
- Slicing is cheap but shares storage — copy or use `s[:n:n]` to isolate.
- The `slices` stdlib package has the common helpers — use it.

Next: **Maps**.

[← Back to roadmap](../roadmap.md)
