package main

import (
	"fmt"
	"net/http"
)

// main route = /users + routing
// 4 default methods = crud

var controller_users = controller{
	endpoints: map[string]endpoint{
		"GET /users":    usersGet,
		"POST /users":   usersCreate,
		"DELETE /users": usersDelete,
		"PUT /users":    usersUpdate,
	},
}

func usersGet(db DB, json func(data any) error) (int, any, error) {
	users, err := db.Users().FindAll()
	if err != nil {
		return 0, nil, err
	}
	type res struct {
		Id    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	results := make([]res, len(users))
	for i, u := range users {
		results[i] = res{Id: u.Id, Name: u.Name, Email: u.Email}
	}

	return http.StatusOK, results, nil
}

func usersDelete(db DB, json func(data any) error) (int, any, error) {
	var req struct {
		Id int `json:"id"  validate:"required"`
	}
	if err := json(&req); err != nil {
		return 0, nil, fmt.Errorf("Cannot DELETE /users\n%w", err)
	}
	err := db.Users().Delete(req.Id)
	if err != nil {
		return 0, nil, fmt.Errorf("Cannot DELETE /users\n%w", err)
	}

	return http.StatusOK, nil, nil
}

func usersCreate(db DB, json func(data any) error) (int, any, error) {
	var req struct {
		Name  string `json:"name"  validate:"required,min=1,max=100"`
		Email string `json:"email" validate:"required,email"`
	}
	if err := json(&req); err != nil {
		return 0, nil, fmt.Errorf("Cannot POST /users\n%w", err)
	}

	user, err := db.Users().Create(User{Name: req.Name, Email: req.Email})
	if err != nil {
		return 0, nil, fmt.Errorf("Cannot POST /users\n%w", err)
	}

	type res struct {
		Id    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	return http.StatusCreated, res{Id: user.Id, Name: user.Name, Email: user.Email}, nil
}

func usersUpdate(db DB, json func(data any) error) (int, any, error) {
	var req struct {
		Id    int    `json:"id"  validate:"required"`
		Name  string `json:"name"  validate:"required,min=1,max=100"`
		Email string `json:"email" validate:"required,email"`
	}
	if err := json(&req); err != nil {
		return 0, nil, fmt.Errorf("Cannot POST /users\n%w", err)
	}

	user, err := db.Users().Update(User{Id: req.Id, Name: req.Name, Email: req.Email})
	if err != nil {
		return 0, nil, fmt.Errorf("Cannot POST /users\n%w", err)
	}

	type res struct {
		Id    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	return http.StatusCreated, res{Id: user.Id, Name: user.Name, Email: user.Email}, nil
}
