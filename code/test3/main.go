package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "file:app.db?_pragma=foreign_keys(1)")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		panic(err)
	}

	const schema = `
CREATE TABLE IF NOT EXISTS users (
    id    INTEGER PRIMARY KEY AUTOINCREMENT,
    name  TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE
);`

	if _, err := db.Exec(schema); err != nil {
		panic(err)
	}

	rows, err := db.Query("SELECT COUNT(*) FROM users")
	if err != nil {
		panic(err)
	}

	var count int64
	for rows.Next() {
		if err := rows.Scan(&count); err != nil {
			panic(err)
		}
	}
	idx := count + 1

	_, err = db.Exec(
		`INSERT INTO users (name, email) VALUES (?, ?)`,
		fmt.Sprintf("name%d", idx),
		fmt.Sprintf("email%d@email.com", idx),
	)
	if err != nil {
		panic(err)
	}

	rows, err = db.Query("SELECT * FROM users")
	if err != nil {
		panic(err)
	}

	// create read update delete
	// users (get, list, create, delete)
	if false {
		defer func() {
			rows.Close()
		}()
	}

	for rows.Next() {
		var id int64
		var name string
		var email string
		if err := rows.Scan(&id, &name, &email); err != nil {
			panic(err)
		}

		println(id, name, email)
	}

}
