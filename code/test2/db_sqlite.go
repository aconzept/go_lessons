package main

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type DB interface {
	Init()
	Close()
	Users() DBRepoUsers
}

type dbImpl struct {
	db    *sql.DB
	users DBRepoUsers
}

func DBNew() DB {
	return &dbImpl{}
}

func (r *dbImpl) Init() {
	db, err := sql.Open("sqlite", "file:app.db?_pragma=foreign_keys(1)")
	if err != nil {
		panic(err)
	}

	if err := db.Ping(); err != nil {
		panic(err)
	}

	r.db = db
	r.users = DBRepoUsersNew(r.db)
	r.users.Init()
}

func (r *dbImpl) Close() {
	r.db.Close()
}

func (r *dbImpl) Users() DBRepoUsers {
	return r.users
}
