package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
)

// Users crud
// Sqlite DB / Repo management
// Absraction for other repos

type jsonWriter struct {
	http.ResponseWriter
}

func (r *jsonWriter) retErr(err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		print(err.Error())
		http.Error(r, apiErr.Text, apiErr.Code)
		return
	}
	http.Error(r, err.Error(), http.StatusBadRequest)
}

type APIError struct {
	Code int
	Text string
}

func (r APIError) Error() string {
	return r.Text
}

func APIErrorNew(code int, Text string) error {
	return &APIError{Code: code, Text: Text}
}

func (r *jsonWriter) retJson(status int, data any) {
	r.Header().Set("Content-Type", "application/json")
	r.WriteHeader(status)
	if err := json.NewEncoder(r).Encode(data); err != nil {
		r.retErr(err)
	}
}

func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&jsonWriter{ResponseWriter: w}, r)
	})
}

func main() {
	db := DBNew()
	db.Init()
	// var a DBRepoUsers
	mainHandler := http.NewServeMux()
	mainHandler.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		jw := w.(*jsonWriter)

		users, err := db.Users().FindAll()
		if err != nil {
			jw.retErr(err)
			return
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

		jw.retJson(http.StatusOK, results)
	})

	mainHandler.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		jw := w.(*jsonWriter)
		var req struct {
			Name  string `json:"name"  validate:"required,min=1,max=100"`
			Email string `json:"email" validate:"required,email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jw.retErr(err)
			return
		}
		if err := validator.New().Struct(req); err != nil {
			jw.retErr(err)
			return
		}

		user, err := db.Users().Create(User{Name: req.Name, Email: req.Email})
		if err != nil {
			jw.retErr(fmt.Errorf("Cannot POST /users\n%w", err))
			return
		}

		type res struct {
			Id    int    `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		jw.retJson(http.StatusCreated, res{
			Id:    user.Id,
			Name:  user.Name,
			Email: user.Email,
		})
	})

	http.ListenAndServe("0.0.0.0:9090", jsonMiddleware(mainHandler))
	db.Close()
}
