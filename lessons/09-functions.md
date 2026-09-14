# Lesson 9 — Functions

> Back to [roadmap](../roadmap.md).

This lesson covers:
- Function declarations and call by value
- Multiple return values
- Named return values
- Variadic functions
- Functions as values, function types
- Anonymous functions and closures
- `defer`

Functions are first-class values: you can assign them to variables, pass them around, store them in slices and maps. There are no classes, no `this`, and no methods on plain functions — methods live on named types (next lesson).

---

## Declaration

```go
func add(a, b int) int {
    return a + b
}
```

Things to notice:

- **Type comes after the name** (`a int`, not `int a`). Same rule everywhere in Go.
- Consecutive parameters of the same type share the type: `(a, b int)` is `a int, b int`.
- Return type comes after the parameter list. For a function with no return value, omit it: `func greet(name string) { ... }`.

Parameters are passed **by value**. The function gets its own copy of each argument. For slices, maps, channels, and pointers this still means "copy the header / the pointer", so the underlying data is shared — but the caller's variable can't be reassigned by the callee.

```go
func zero(x int)       { x = 0 }   // doesn't affect the caller
func zeroPtr(x *int)   { *x = 0 }  // does
```

---

## Multiple return values

A Go function can return any number of values. The canonical use is "result plus error":

```go
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, errors.New("divide by zero")
    }
    return a / b, nil
}

q, err := divide(10, 3)
if err != nil { return err }
fmt.Println(q)
```

You can discard a returned value with `_`:

```go
_, err := divide(10, 3)
```

This pattern — return both result and error, check the error immediately — is *everywhere* in Go. It's noisier than exceptions but always visible in the diff: every place an operation can fail, there's a checked `err`.

---

## Named return values

Returns can be **named**, in which case they're pre-declared local variables and a bare `return` implicitly returns them:

```go
func split(sum int) (x, y int) {
    x = sum * 4 / 9
    y = sum - x
    return                  // returns x, y
}
```

Use sparingly. Idiomatic uses:

1. **Documentation.** When the meaning of each return value isn't obvious from the type:
   ```go
   func parseHost(s string) (host string, port int, err error) { ... }
   ```
2. **`defer` needs to modify the return value** — typically to wrap an error or recover from a panic:
   ```go
   func work() (err error) {
       defer func() {
           if r := recover(); r != nil {
               err = fmt.Errorf("panic: %v", r)
           }
       }()
       // ...
       return
   }
   ```

In normal code, prefer explicit `return x, y` — bare returns get hard to follow once the function is longer than a screen.

---

## Variadic functions

A function whose last parameter has `...T` accepts any number of arguments of type `T` (zero or more):

```go
func sum(nums ...int) int {
    total := 0
    for _, n := range nums {
        total += n
    }
    return total
}

sum()              // 0
sum(1, 2, 3)       // 6
```

Inside the function, `nums` is a `[]int`. You can also pass an existing slice with `...`:

```go
xs := []int{1, 2, 3}
sum(xs...)         // spread
```

The most common variadic in real code is `fmt.Printf(format string, args ...any)` — you just keep adding arguments.

---

## Functions as values

A function value can be stored, passed, and called. The type signature is just the keyword `func` minus the name:

```go
var op func(int, int) int = add
fmt.Println(op(2, 3))   // 5
```

This is how you write callbacks, middleware, and the "strategy" pattern:

```go
func apply(xs []int, f func(int) int) []int {
    out := make([]int, len(xs))
    for i, x := range xs {
        out[i] = f(x)
    }
    return out
}

doubled := apply([]int{1, 2, 3}, func(n int) int { return n * 2 })
```

If a function type recurs, give it a name:

```go
type Predicate func(int) bool

func filter(xs []int, keep Predicate) []int { ... }
```

`http.HandlerFunc` is a real-world example — a named function type with a method on it.

---

## Anonymous functions and closures

A function literal can be called inline or assigned:

```go
inc := func(n int) int { return n + 1 }
inc(4)   // 5

// Immediately invoked
func() {
    fmt.Println("once")
}()
```

A function literal **closes over** the variables in its lexical scope. The variables are shared, not copied:

```go
func makeCounter() func() int {
    count := 0
    return func() int {
        count++
        return count
    }
}

c := makeCounter()
c()   // 1
c()   // 2
c()   // 3
```

Each call to `makeCounter` creates a fresh `count`. The returned function lives as long as anything references it, and so does the variable it captured — the GC handles that.

This is the same closure semantics you have in JS/TS. The one historical gotcha (the loop-variable capture) was fixed in Go 1.22, as covered in the previous lesson.

---

## `defer`

`defer` schedules a function call to run **when the surrounding function returns** — whether by `return`, by reaching the end, or by panicking.

```go
func readFile(path string) ([]byte, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    return io.ReadAll(f)
}
```

`f.Close()` runs after `io.ReadAll` returns, even on the error path. This is the Go idiom for "always clean up": pair every resource acquisition with a `defer` to release it, right next to each other.

Rules worth knowing:

- **Arguments are evaluated at the `defer` statement**, not at call time:
  ```go
  x := 10
  defer fmt.Println(x)   // prints 10
  x = 99
  ```
  If you need late binding, wrap in a closure: `defer func() { fmt.Println(x) }()`.
- **LIFO order.** Multiple `defer`s run in reverse order.
- A deferred call **can read and modify named return values** — that's the recover-and-wrap pattern shown earlier.
- `defer` adds tiny overhead (negligible outside hot loops). In normal code, prefer clarity.

---

## Putting it together — a function tour

```go
type Result struct {
    Total int
    Avg   float64
}

func summarize(nums ...int) (Result, error) {
    if len(nums) == 0 {
        return Result{}, errors.New("no numbers")
    }
    sum := 0
    for _, n := range nums {
        sum += n
    }
    return Result{
        Total: sum,
        Avg:   float64(sum) / float64(len(nums)),
    }, nil
}

func main() {
    r, err := summarize(2, 4, 6)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("%+v\n", r)   // {Total:12 Avg:4}
}
```

Variadic input, two returns, struct result, explicit error check, `%+v` to print fields with names. This shape is the bread and butter of Go.

---

## Recap

- Type goes after the name; parameters by value; multiple return values are common.
- Named returns are a documentation/`defer`-cooperation tool — don't use bare `return` casually.
- Variadic last parameter is `...T`; spread an existing slice with `xs...`.
- Functions are first-class values; closures work as in TS; loop-capture is no longer a gotcha on Go 1.22+.
- `defer` runs at function exit, in LIFO order, with arguments captured at the defer site — the idiomatic place for cleanup.

Next: **Pointers**.

[← Back to roadmap](../roadmap.md)
