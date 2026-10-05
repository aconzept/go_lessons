package main

import (
	"database/sql"
	"fmt"
	"strings"
)

type User struct {
	Id    int
	Name  string
	Email string
}

type DBRepoUsers interface {
	Init()
	FindAll() []User
	FindBy(u User) []User
	Create(u User) (User, error)
}

type usersImpl struct {
	db *sql.DB
}

func DBRepoUsersNew(db *sql.DB) DBRepoUsers {
	return &usersImpl{db: db}
}

func (r *usersImpl) Init() {
	const schema = `
CREATE TABLE IF NOT EXISTS users (
    id    INTEGER PRIMARY KEY AUTOINCREMENT,
    name  TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE
);`
	if _, err := r.db.Exec(schema); err != nil {
		panic(err)
	}
}

func (r *usersImpl) FindAll() []User {
	rows, err := r.db.Query("SELECT * FROM users")
	if err != nil {
		panic(err)
	}
	var result []User
	for rows.Next() {
		var id int
		var name string
		var email string
		if err := rows.Scan(&id, &name, &email); err != nil {
			panic(err)
		}

		result = append(result, User{Id: id, Name: name, Email: email})
	}
	return result
}

func (r *usersImpl) FindBy(u User) []User {
	queryConstraints := []string{}
	if u.Id > 0 {
		queryConstraints = append(queryConstraints, fmt.Sprintf(" id = %d ", u.Id))
	}
	if len(u.Name) > 0 {
		queryConstraints = append(queryConstraints, fmt.Sprintf(" name = \"%s\" ", u.Name))
	}
	if len(u.Email) > 0 {
		queryConstraints = append(queryConstraints, fmt.Sprintf(" email = \"%s\" ", u.Email))
	}
	queryConstraintsString := strings.Join(queryConstraints, " AND ")
	query := fmt.Sprintf(" SELECT * FROM users WHERE %s ", queryConstraintsString)

	rows, err := r.db.Query(query)
	if err != nil {
		panic(err)
	}
	var result []User
	// if err = rows.Err(); err != nil {
	// 	panic(err)
	// }
	for rows.Next() {
		var id int
		var name string
		var email string
		if err := rows.Scan(&id, &name, &email); err != nil {
			panic(err)
		}

		result = append(result, User{Id: id, Name: name, Email: email})
	}
	return result
}

func (r *usersImpl) Create(u User) (User, error) {
	res, err := r.db.Exec(
		`INSERT INTO users (name, email) VALUES (?, ?)`,
		u.Name,
		u.Email,
	)
	if err != nil {
		return User{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}

	return User{Id: int(id), Name: u.Name, Email: u.Email}, nil
}
