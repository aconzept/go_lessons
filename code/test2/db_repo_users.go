package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type User struct {
	Id    int
	Name  string
	Email string
}

type DBRepoUsers interface {
	Init()
	FindAll() ([]User, error)
	FindBy(u User) ([]User, error)
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

func (r *usersImpl) FindAll() ([]User, error) {
	return r.FindBy(User{})
}

func (r *usersImpl) FindBy(u User) ([]User, error) {
	queryConstraints := []string{}
	queryParams := []any{}
	if u.Id > 0 {
		queryConstraints = append(queryConstraints, " id = ? ")
		queryParams = append(queryParams, u.Id)
	}
	if len(u.Name) > 0 {
		queryConstraints = append(queryConstraints, " name = ? ")
		queryParams = append(queryParams, u.Name)
	}
	if len(u.Email) > 0 {
		queryConstraints = append(queryConstraints, " email = ? ")
		queryParams = append(queryParams, u.Email)
	}
	queryConstraintsString := ""
	if len(queryConstraints) > 0 {
		queryConstraintsString = fmt.Sprintf(" WHERE %s ", strings.Join(queryConstraints, " AND "))
	}
	query := fmt.Sprintf(" SELECT * FROM users %s ", queryConstraintsString)

	rows, err := r.db.Query(query, queryParams...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *usersImpl) Create(u User) (User, error) {
	res, err := r.db.Exec(
		` INSERT INTO users (name, email) VALUES (?, ?) `,
		u.Name,
		u.Email,
	)
	if err != nil {
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) {
			if sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
				return User{}, fmt.Errorf("Cannot DBRepoUsers.Create\n%w\n%w",
					APIErrorNew(http.StatusConflict, fmt.Sprintf("Email %s already taken", u.Email)),
					err,
				)
			}
		}

		return User{}, fmt.Errorf("Cannot DBRepoUsers.Create\n%w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("Cannot DBRepoUsers.Create\n%w", err)
	}
	users, err := r.FindBy(User{Id: int(id)})
	if err != nil {
		return User{}, fmt.Errorf("Cannot DBRepoUsers.Create\n%w", err)
	}
	if len(users) != 1 {
		return User{}, fmt.Errorf("not unique user")
	}

	return users[0], fmt.Errorf("Cannot DBRepoUsers.Create\n%w", err)
}
