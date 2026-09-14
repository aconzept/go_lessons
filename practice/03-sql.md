# Practice 3 — SQL with `database/sql`

> Back to [roadmap](../roadmap.md).

This lesson covers:
- Why `database/sql` needs a driver package even though it's "no ORM, no extra packages"
- Opening a database, and why `sql.Open` doesn't actually connect
- Connection pool settings
- `Query`, `QueryRow`, `Exec` — and the `rows.Close()`/`rows.Err()` you must not skip
- Scanning into structs, and handling `NULL`
- Parameterised queries — and why the placeholder syntax isn't portable
- Transactions
- A complete worked example

`database/sql` is a generic, driver-agnostic interface for talking to a SQL database. It does not map structs to tables, generate migrations, or build queries for you — that's what packages like `gorm`, `sqlx`, or `ent` are for. This lesson is the plain stdlib layer everything else is built on.

---

## The driver is the one package you can't avoid

`database/sql` defines interfaces (`driver.Driver`, `driver.Conn`, `driver.Rows`, ...) but ships no implementation of any of them — there is no wire protocol for Postgres or MySQL or SQLite baked into the standard library. You register a driver package, which implements those interfaces and wires itself in via a blank import:

```go
import (
    "database/sql"

    _ "modernc.org/sqlite"   // registers itself with database/sql; nothing else is used directly
)
```

`modernc.org/sqlite` is a pure-Go SQLite driver — no cgo, no C compiler needed, which makes it the simplest thing to run in this repo's [devcontainer](01-docker-and-vscode-debugging.md) or anywhere else. For a real server you'd reach for `github.com/jackc/pgx` (Postgres) or `github.com/go-sql-driver/mysql` instead — same `database/sql` API, different import and connection string.

The blank import (`_`) is the same idiom as any package imported purely for its `init()` side effect ([Lesson 14](../lessons/14-code-organization.md)) — the driver registers itself with `sql.Register` and you never call its exported names directly.

```
go get modernc.org/sqlite
```

---

## Opening a database

```go
db, err := sql.Open("sqlite", "file:app.db?_pragma=foreign_keys(1)")
if err != nil {
    return fmt.Errorf("open db: %w", err)
}
defer db.Close()
```

**`sql.Open` does not connect.** It validates the arguments and sets up an idle connection pool, lazily — the first real connection happens on first use. To fail fast at startup instead of on the first request, ping it explicitly:

```go
if err := db.Ping(); err != nil {
    return fmt.Errorf("ping db: %w", err)
}
```

`*sql.DB` is not "a connection" — it's a pool. It's safe for concurrent use from many goroutines, and you create exactly one per database for the life of the program, never one per request.

### Pool settings

```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(25)
db.SetConnMaxLifetime(5 * time.Minute)
```

Defaults are unlimited open connections and no lifetime cap — fine for a demo, a good way to exhaust a real database's connection limit under load. `SetConnMaxLifetime` matters most behind a load balancer or proxy (PgBouncer, cloud SQL proxies) that silently drops long-idle connections out from under you.

---

## Schema, and running DDL

```go
const schema = `
CREATE TABLE IF NOT EXISTS users (
    id    INTEGER PRIMARY KEY AUTOINCREMENT,
    name  TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE
);`

if _, err := db.Exec(schema); err != nil {
    return fmt.Errorf("migrate: %w", err)
}
```

`Exec` is for statements that don't return rows — DDL, `INSERT`, `UPDATE`, `DELETE`. Nothing here manages migrations for you; a real project reaches for `golang-migrate` or `goose` once there's more than one schema change to track, but the underlying calls are the same `Exec`.

---

## Querying

### One row

```go
func GetUser(db *sql.DB, id int64) (User, error) {
    var u User
    err := db.QueryRow(
        `SELECT id, name, email FROM users WHERE id = ?`, id,
    ).Scan(&u.ID, &u.Name, &u.Email)

    if errors.Is(err, sql.ErrNoRows) {
        return User{}, fmt.Errorf("user %d: %w", id, ErrNotFound)
    }
    if err != nil {
        return User{}, fmt.Errorf("get user %d: %w", id, err)
    }
    return u, nil
}
```

`QueryRow` never returns an error you check before `Scan` — the error (including "no rows") only surfaces from `Scan` itself. `sql.ErrNoRows` is a sentinel ([Lesson 15](../lessons/15-error-handling.md)): expected, not exceptional — check it with `errors.Is`, don't let it read as "the database broke."

### Many rows

```go
func ListUsers(db *sql.DB) ([]User, error) {
    rows, err := db.Query(`SELECT id, name, email FROM users ORDER BY id`)
    if err != nil {
        return nil, fmt.Errorf("list users: %w", err)
    }
    defer rows.Close()

    var out []User
    for rows.Next() {
        var u User
        if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
            return nil, fmt.Errorf("scan user: %w", err)
        }
        out = append(out, u)
    }
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("list users: %w", err)
    }
    return out, nil
}
```

Three things people skip and later regret:

1. **`defer rows.Close()`.** Skip it and you leak a connection out of the pool every time — the pool quietly shrinks until every request blocks waiting for one. Calling it twice is harmless; `rows.Close()` is idempotent.
2. **`rows.Err()` after the loop.** `rows.Next()` returning `false` means either "no more rows" *or* "an error broke the scan partway through" — those look identical unless you check `Err()`.
3. **`Scan` positions map to `SELECT` column order**, not struct field order or name. `SELECT *` is fragile here for exactly that reason — name your columns.

There is no `Scan(&u)` that fills a whole struct from a row — that struct-mapping convenience is precisely the slice of functionality an ORM (or the lighter `sqlx`) adds on top.

---

## `NULL`

A `string` field can't hold SQL `NULL` — `Scan` returns an error if it tries. Two ways to handle a nullable column:

```go
type User struct {
    ID    int64
    Name  string
    Email string
    Bio   sql.NullString   // nullable column
}

err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Bio)
if u.Bio.Valid {
    fmt.Println(u.Bio.String)
}
```

Or scan into a pointer instead of a stdlib `NullXxx` type — `nil` for `NULL`, a valid address otherwise:

```go
var bio *string
err := row.Scan(&u.ID, &u.Name, &u.Email, &bio)
if bio != nil {
    u.Bio = *bio
}
```

Pointers read more naturally with `encoding/json` downstream (`omitempty` on a `nil` pointer just omits the field); `sql.NullXxx` is more explicit about "this is a nullable DB column" at the type level. Either is idiomatic — pick one convention and hold to it across a codebase.

---

## Parameterised queries — and the placeholder trap

```go
db.QueryRow(`SELECT * FROM users WHERE email = ?`, email)   // SQLite, MySQL
db.QueryRow(`SELECT * FROM users WHERE email = $1`, email)  // Postgres (pgx, lib/pq)
```

**Always** pass values as parameters, never `fmt.Sprintf` them into the query string — that's SQL injection, full stop. The driver is what decides the placeholder syntax (`?` vs `$1` vs `:name`), not `database/sql` itself, which is why switching drivers can mean rewriting every query string even though the Go code around it barely changes. This is one of the concrete costs of going driver-agnostic that something like `sqlx` or an ORM tries to paper over — `database/sql` deliberately doesn't.

`Exec` for a write, same rule:

```go
res, err := db.Exec(
    `INSERT INTO users (name, email) VALUES (?, ?)`, name, email)
if err != nil {
    return fmt.Errorf("insert user: %w", err)
}
id, _ := res.LastInsertId()   // driver-dependent: not all drivers support this (pgx doesn't)
n, _ := res.RowsAffected()
```

`LastInsertId` is a MySQL/SQLite-ism baked into the `driver.Result` interface for historical reasons; Postgres drivers commonly return `ErrNoLastInsertID` for it — use `RETURNING id` in the query and `Scan` it instead when you need the new ID on Postgres.

---

## Transactions

```go
func TransferCredits(db *sql.DB, fromID, toID int64, amount int) error {
    tx, err := db.Begin()
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback()   // no-op if Commit already succeeded

    if _, err := tx.Exec(
        `UPDATE accounts SET balance = balance - ? WHERE id = ?`, amount, fromID); err != nil {
        return fmt.Errorf("debit: %w", err)
    }
    if _, err := tx.Exec(
        `UPDATE accounts SET balance = balance + ? WHERE id = ?`, amount, toID); err != nil {
        return fmt.Errorf("credit: %w", err)
    }

    return tx.Commit()
}
```

`defer tx.Rollback()` immediately after a successful `BeginTx` is the pattern, every time — it's a safety net, not the primary control flow. Once `Commit()` succeeds, the deferred `Rollback()` is a documented no-op (`sql.ErrTxDone`, discarded). If any step above returns early, the deferred rollback is what actually undoes the first `UPDATE`.

A `*sql.Tx` is bound to a single connection from the pool — every call in the transaction must go through `tx`, not the original `db`, or it silently runs outside the transaction on a different connection.

---

## A complete example

```go
package main

import (
    "database/sql"
    "errors"
    "fmt"
    "log/slog"
    "os"

    _ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

type User struct {
    ID    int64
    Name  string
    Email string
}

func main() {
    if err := run(); err != nil {
        slog.Error("failed", "err", err)
        os.Exit(1)
    }
}

func run() error {
    db, err := sql.Open("sqlite", "file:app.db")
    if err != nil {
        return fmt.Errorf("open db: %w", err)
    }
    defer db.Close()

    if err := db.Ping(); err != nil {
        return fmt.Errorf("ping db: %w", err)
    }

    if _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id    INTEGER PRIMARY KEY AUTOINCREMENT,
            name  TEXT NOT NULL,
            email TEXT NOT NULL UNIQUE
        )`); err != nil {
        return fmt.Errorf("migrate: %w", err)
    }

    res, err := db.Exec(
        `INSERT INTO users (name, email) VALUES (?, ?)`, "Ada", "ada@example.com")
    if err != nil {
        return fmt.Errorf("insert user: %w", err)
    }
    id, _ := res.LastInsertId()

    var u User
    err = db.QueryRow(
        `SELECT id, name, email FROM users WHERE id = ?`, id,
    ).Scan(&u.ID, &u.Name, &u.Email)
    if err != nil {
        return fmt.Errorf("get user: %w", err)
    }

    fmt.Printf("inserted: %+v\n", u)
    return nil
}
```

Try it:

```bash
go run .
# inserted: {ID:1 Name:Ada Email:ada@example.com}

go run .
# Error: insert user: UNIQUE constraint failed: users.email
```

Same idea as [Practice 1](01-docker-and-vscode-debugging.md): put a breakpoint on the `Scan` call, F5, and inspect `u` field by field once the query returns.

---

## Gotchas

| Symptom | Cause | Fix |
|---|---|---|
| `sql: unknown driver "..." (forgotten import?)` | Driver package imported but not blank-imported, or not imported at all | `import _ "modernc.org/sqlite"` |
| Errors only show up on first query, not at startup | `sql.Open` doesn't connect | Call `db.Ping()` right after opening |
| Pool exhausted under load; requests hang | Missing `rows.Close()` on some path | `defer rows.Close()` immediately after a successful `Query` |
| Rows silently stop partway with no error shown | `rows.Next()` loop ended on an error, not just "done" | Check `rows.Err()` after the loop |
| `converting NULL to string is unsupported` | Scanning a nullable column into a plain `string` | Use `sql.NullString` or a `*string` |
| Query works in one driver, breaks in another with the same code | Placeholder syntax (`?` vs `$1`) is driver-specific | Match the placeholder style to the driver you're using |
| `res.LastInsertId()` returns an error | Postgres drivers commonly don't support it | Use `INSERT ... RETURNING id` and `Scan` it |
| A write inside a transaction doesn't roll back with the rest | Called `db.Exec` instead of `tx.Exec` | Every statement in the transaction must go through `tx` |
| `database is locked` (SQLite specifically) | Concurrent writers; SQLite allows one writer at a time | Keep `SetMaxOpenConns(1)` for SQLite, or move to a server-based DB under real concurrency |

---

## Recap

- `database/sql` is an interface; a driver package (blank-imported) provides the implementation. There is no driver-free way to use it.
- `sql.Open` doesn't connect — `Ping` is what actually verifies the database is reachable.
- `*sql.DB` is a pool, not a connection: one per database, shared across goroutines, tuned with `SetMaxOpenConns`/`SetConnMaxLifetime`.
- `QueryRow` + `Scan` for one row (watch for `sql.ErrNoRows`); `Query` + `rows.Next()`/`Scan`/`rows.Close()`/`rows.Err()` for many.
- Nullable columns need `sql.NullString`-style types or pointers — a bare `string` can't hold `NULL`.
- Parameters always go through placeholders, never string formatting — and the placeholder syntax is driver-specific, not portable.
- `defer tx.Rollback()` right after `BeginTx`; every statement in a transaction must go through `tx`, not `db`.

[← Back to roadmap](../roadmap.md)
