package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
)

type endpoint = func(db DB, json func(data any) error) (int, any, error)

type controller struct {
	endpoints map[string]endpoint
}

func (r *controller) readJson(httpReq *http.Request, req any) error {
	if err := json.NewDecoder(httpReq.Body).Decode(&req); err != nil {
		return err
	}
	if err := validator.New().Struct(req); err != nil {
		return err
	}
	return nil
}

func (r *controller) retJson(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		r.retErr(w, err)
	}
}

func (r *controller) retErr(w http.ResponseWriter, err error) {
	var apiErr *APIError
	print(err.Error())
	if errors.As(err, &apiErr) {
		http.Error(w, apiErr.Text, apiErr.Code)
		return
	}

	http.Error(w, "Something went wrong", http.StatusInternalServerError)
}
