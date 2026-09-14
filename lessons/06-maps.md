# Lesson 6 — Maps

> Back to [roadmap](../roadmap.md).

This lesson covers:
- Declaring and constructing maps
- Reads, writes, deletes
- The comma-ok idiom
- Iteration and ordering
- Maps as reference types
- Which key types are allowed
- When to reach for a struct instead

A Go `map` is a hash table. It's the closest analogue to a JS object used as a dictionary, but unlike a JS object, a Go map has one key type and one value type and nothing else.

---

## Declaring and constructing

```go
// Literal
prices := map[string]float64{
    "apple":  1.20,
    "banana": 0.30,
}

// Empty, ready to use
counts := make(map[string]int)
counts["hits"]++   // counts now == {"hits": 1}

// Zero value: nil. Reading is fine, writing panics.
var m map[string]int
_ = m["x"]      // OK — returns 0
m["x"] = 1      // PANIC: assignment to entry in nil map
```

The `var m map[K]V` form gives you a nil map. You can read from it (you'll get zero values) but you cannot write. **Use `make` (or a literal) before writing.** This trips people up — when in doubt, `make`.

---

## Reads, writes, deletes

```go
m := map[string]int{"a": 1, "b": 2}

m["c"] = 3            // insert
m["a"] = 100          // update — same syntax
v := m["a"]           // read

delete(m, "a")        // remove (no error if absent)
n := len(m)           // number of entries
```

Reading a missing key returns the **zero value** of the value type, silently:

```go
m := map[string]int{}
fmt.Println(m["nope"])   // 0
```

This is great for counters (`m[k]++` works whether `k` is present or not) and bad when you actually need to know "was this key set?". For that, use the comma-ok idiom.

---

## The comma-ok idiom

A two-value read returns the value *and* a boolean indicating presence:

```go
v, ok := m["maybe"]
if !ok {
    // key was not in the map
}
```

This is the only way to distinguish "key absent" from "key present with the zero value". It shows up in three places in the language and you'll see all of them:

```go
v, ok := m[k]        // map read
v, ok := <-ch        // channel receive (ok = false means closed)
v, ok := x.(T)       // type assertion (ok = false means wrong type)
```

Same syntactic pattern in all three. When you see `, ok :=` in someone else's code, you now know what's happening.

---

## Iteration

`for range` over a map yields `(key, value)`:

```go
for k, v := range prices {
    fmt.Println(k, v)
}

for k := range prices { ... }     // keys only
```

**Order is unspecified and intentionally randomized between runs.** Don't rely on it. If you need a stable order, extract the keys, sort them, and iterate:

```go
keys := make([]string, 0, len(prices))
for k := range prices {
    keys = append(keys, k)
}
sort.Strings(keys)
for _, k := range keys {
    fmt.Println(k, prices[k])
}
```

You can `delete` entries during iteration safely. Inserting during iteration is allowed but the new entries may or may not be visited — don't rely on it.

---

## Maps are reference-ish

A map value is a small header pointing at the underlying hash table. Passing a map to a function does *not* copy its contents — both the caller and the callee see the same entries:

```go
func add(m map[string]int) {
    m["new"] = 1
}

m := map[string]int{}
add(m)
fmt.Println(m["new"])   // 1
```

So if you want a function to mutate the caller's map, just pass it — no pointer needed. (Compare with slices, where you also need to reassign the result of `append`.)

To copy a map, iterate and assign, or use `maps.Clone` (Go 1.21+):

```go
import "maps"
copyOfM := maps.Clone(m)
```

The `maps` package also has `Equal`, `Keys`, `Values`, and `Copy` — worth a skim before hand-rolling loops.

---

## Which keys are allowed

A map key must be **comparable** with `==`. That includes:

- All basic types: numbers, strings, booleans.
- Pointers, channels.
- Interfaces (compared by dynamic type + value).
- Structs and arrays — *if* all their fields/elements are comparable.

It does **not** include slices, maps, or functions — those are not comparable. Trying to use one as a key is a compile error:

```go
m := map[[]string]int{}    // ERROR: invalid map key type
```

A struct of basic fields as a composite key is idiomatic and fast:

```go
type cell struct{ x, y int }
grid := map[cell]string{}
grid[cell{1, 2}] = "A"
```

This is the Go way to do what JS code sometimes does with `${x},${y}` string keys — but typed, allocation-free, and with `==` doing the right thing.

---

## When to use a map vs a struct

Maps are for **dynamic, unbounded sets of keys** discovered at runtime — request headers, configuration tables, caches, counters.

If the keys are **known and fixed** ahead of time, use a struct:

```go
// Not idiomatic
user := map[string]any{
    "id":   42,
    "name": "Ada",
}

// Idiomatic
type User struct {
    ID   int
    Name string
}
user := User{ID: 42, Name: "Ada"}
```

Structs give you per-field types, zero-cost access, autocomplete, and a place to hang methods. Reach for a map only when the keys really are data.

---

## Concurrency note

A Go map is **not safe for concurrent use** if any goroutine is writing. The runtime detects races and panics outright (it's deliberate — silent corruption would be worse). When you need concurrent access:

- A `sync.RWMutex` around a normal map (most common).
- `sync.Map` from the standard library — optimised for "write-once, read-many" or sharded keys, not a drop-in faster map.

Concurrency comes later in the roadmap. For now, just don't share a map across goroutines without protection.

---

## Recap

- A map is `map[K]V`. Construct with a literal or `make`; a `nil` map can be read but panics on write.
- Missing keys read as the zero value; use `v, ok := m[k]` to test presence.
- Iteration order is randomized — sort keys if you need stable output.
- Maps are passed by reference; you don't need pointers to mutate them in a callee.
- Keys must be comparable — structs of basic fields are fine, slices and maps are not.
- For fixed shapes, use a struct, not a map.

Next: **Structs** — fields, methods, tags, and embedding.

[← Back to roadmap](../roadmap.md)
