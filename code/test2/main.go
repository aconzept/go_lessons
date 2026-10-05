package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
)

// Users crud
// Sqlite DB / Repo management
// Absraction for other repos

func main() {
	db := DBNew()
	db.Init()
	// var a DBRepoUsers
	mainHandler := http.NewServeMux()
	mainHandler.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		users := db.Users().FindAll()
		type res struct {
			Id    int    `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		results := make([]res, len(users))
		for i, u := range users {
			results[i] = res{Id: u.Id, Name: u.Name, Email: u.Email}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(results); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

	})

	mainHandler.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name  string `json:"name"  validate:"required,min=1,max=100"`
			Email string `json:"email" validate:"required,email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validator.New().Struct(req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		user, err := db.Users().Create(User{Name: req.Name, Email: req.Email})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(user); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

	})

	http.ListenAndServe("0.0.0.0:9090", mainHandler)
	db.Close()
}
