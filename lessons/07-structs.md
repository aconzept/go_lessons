# Lesson 7 — Structs

> Back to [roadmap](../roadmap.md).

This lesson covers:
- Defining and using structs
- Value vs pointer semantics
- Struct tags — and how `encoding/json` uses them
- Anonymous structs and field
- Embedding (Go's answer to mixins/inheritance)
- Comparability and equality

Structs are the workhorse type in Go. Almost every domain model, request/response, and configuration is a struct.

---

## Defining structs

```go
type User struct {
    ID        int
    Name      string
    Email     string
    CreatedAt time.Time
}
```

A struct is just a typed record — like an interface in TypeScript, but **nominal** (the name matters, not just the shape).

Construct with a literal:

```go
// Named fields — preferred
u := User{
    ID:   1,
    Name: "Ada",
}

// Positional — works but fragile. Don't use in app code.
u := User{1, "Ada", "ada@example.com", time.Now()}

// Zero value — every field at its zero value
var empty User
```

Unmentioned fields get their zero values, so `User{ID: 1}` is fine.

Access fields with `.`:

```go
u.Name = "Lovelace"
```

### Exported vs unexported

A field whose name **starts with an uppercase letter** is exported (visible to other packages). Lowercase fields are package-private.

```go
type Account struct {
    ID      int       // exported
    balance float64   // package-private — JSON and reflection skip it too
}
```

This is the only access-control mechanism Go has — there is no `private` keyword. It's coarse but consistent.

---

## Value vs pointer

A struct is a value type. Assigning, returning, or passing a struct **copies every field**:

```go
a := User{Name: "Ada"}
b := a
b.Name = "Lovelace"
fmt.Println(a.Name)   // Ada
```

If you want a function to mutate the caller's struct, pass a pointer:

```go
func upgrade(u *User) {
    u.Name = "Lovelace"
}

u := User{Name: "Ada"}
upgrade(&u)
fmt.Println(u.Name)   // Lovelace
```

Note: when you have a `*User`, you write `p.Name` — not `(*p).Name`. Go auto-dereferences on field access.

**When to pass by value vs by pointer:**
- Small structs (a few fields, no slices/maps) → value is fine and often faster.
- Large structs → pointer to avoid copying.
- You need to mutate the original → pointer.
- The type has methods with pointer receivers (next lesson) → use a pointer to it consistently.

In practice, most non-trivial domain types end up being passed as `*T`. Don't over-think it early — start with values, switch to pointers when there's a reason.

### Constructors

There's no `new` operator the way you'd use in TS. The conventions:

```go
// 1. Literal — most common
u := User{ID: 1, Name: "Ada"}

// 2. The built-in `new` returns *T pointing at a zeroed T
p := new(User)        // *User, every field zero

// 3. Custom constructor — convention is NewX
func NewUser(name string) *User {
    return &User{
        ID:        nextID(),
        Name:      name,
        CreatedAt: time.Now(),
    }
}
```

`&User{...}` is the idiomatic way to get a `*User` literal in one step. You'll see it everywhere.

---

## Struct tags

A field can carry a **tag** — a string of metadata that reflection-based libraries (`encoding/json`, validators, ORMs) read at runtime:

```go
type User struct {
    ID       int       `json:"id"`
    Name     string    `json:"name"`
    Email    string    `json:"email,omitempty"`
    Password string    `json:"-"`
    Joined   time.Time `json:"joined_at"`
}
```

Tag syntax is `key:"value" key2:"value2"` — backticks around the whole thing (so you don't have to escape the inner `"`).

What `encoding/json` understands:

| Tag | Effect |
|---|---|
| `json:"name"` | Use `name` as the JSON key instead of the field name. |
| `json:"name,omitempty"` | Skip the field entirely when its value is the zero value. |
| `json:"-"` | Never marshal/unmarshal this field. |
| `json:",string"` | Encode a numeric field as a JSON string (for big ints, etc.). |

Other ecosystems use their own tag namespace: `db:"..."` for sqlx, `gorm:"..."` for GORM, `validate:"..."` for go-playground/validator, `yaml:"..."` for yaml.v3, and so on. A field can carry several at once:

```go
ID int `json:"id" db:"id" validate:"required"`
```

Tags are just strings — typos compile fine and fail silently at runtime. A few minutes spent learning what `omitempty` actually does (it triggers on the zero value, including `0`, `""`, `false`, and `nil`) saves real bugs later.

### Encoding & decoding

```go
import "encoding/json"

u := User{ID: 1, Name: "Ada", Email: "ada@example.com"}
b, err := json.Marshal(u)
// b = `{"id":1,"name":"Ada","email":"ada@example.com","joined_at":"0001-..."}`

var u2 User
err = json.Unmarshal(b, &u2)   // note the &
```

Unmarshal needs a pointer to fill in. Fields not present in the JSON keep their zero values — Unmarshal doesn't error on missing keys, only on type mismatches.

---

## Anonymous structs

You can declare a struct type inline, without naming it:

```go
config := struct {
    Host string
    Port int
}{
    Host: "localhost",
    Port: 8080,
}
```

Useful for one-off return shapes, test fixtures, and table-driven tests:

```go
cases := []struct {
    name string
    in   int
    want int
}{
    {"zero",   0, 0},
    {"double", 3, 6},
}
```

If you find yourself writing the same anonymous struct in two places, give it a name.

---

## Embedding

Embedding is Go's mechanism for composition. Instead of inheritance, you put one struct **inside** another **without giving it a field name**, and its fields/methods promote to the outer struct.

```go
type Timestamps struct {
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Post struct {
    Timestamps        // embedded, no field name
    ID    int
    Title string
}

p := Post{}
p.CreatedAt = time.Now()   // promoted from Timestamps
p.Timestamps.CreatedAt = time.Now()  // also valid — explicit access
```

The embedded type's name (`Timestamps`) is also a valid field name when you need to refer to the whole inner struct.

You can embed **types you don't own**, including pointers and interfaces:

```go
type LoggingClient struct {
    *http.Client     // pointer embedding — methods of *http.Client are promoted
    log *slog.Logger
}

c := &LoggingClient{Client: http.DefaultClient, log: ...}
resp, err := c.Get(url)   // c.Get is *http.Client.Get
```

This is the standard pattern for "wrap a thing and add behaviour" — closer to decorators or mixins than to subclassing. There is **no inheritance, no virtual dispatch, no `super`**. If the outer struct defines its own method with the same name, it shadows the inner one but doesn't override it in any polymorphic sense.

Use embedding when it actually expresses "is a kind of / wraps a kind of". For pure code reuse, a plain field is usually clearer.

---

## Comparability

A struct is comparable with `==` if and only if every field is comparable:

```go
type Point struct{ X, Y int }
Point{1, 2} == Point{1, 2}   // true

type Bad struct{ Items []int }
Bad{} == Bad{}               // compile error — slice not comparable
```

This matters for:
- Using a struct as a map key (it must be comparable).
- Equality assertions in tests (use `reflect.DeepEqual` or `cmp.Diff` when fields include slices/maps).

---

## Recap

- Struct = typed record. Use named-field literals; `&User{...}` for a pointer.
- Field starts uppercase → exported; lowercase → package-private. Same rule for everything in Go.
- Pass by value for small things, pointer for large or mutable ones; the type's method-receiver style usually dictates.
- Struct tags drive JSON/DB/validation libraries. They're just strings; learn `omitempty` and `-` for `encoding/json`.
- Embedding composes types and promotes fields/methods — it is **not** inheritance.
- A struct of comparable fields is itself comparable and can be a map key.

Next: **Loops & Conditionals**.

[← Back to roadmap](../roadmap.md)
