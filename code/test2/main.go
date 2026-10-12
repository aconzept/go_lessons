package main

import (
	"errors"
	"fmt"
	"net/http"
)

// Users crud
// Sqlite DB / Repo management
// Absraction for other repos

type jsonWriter struct {
	http.ResponseWriter
}

func (r *jsonWriter) retErr(err error) {
	var apiErr *APIError
	print(err.Error())
	if errors.As(err, &apiErr) {
		http.Error(r, apiErr.Text, apiErr.Code)
		return
	}

	http.Error(r, "Something went wrong", http.StatusInternalServerError)
}

// Guard = compute + chain
// cognito - token -> APIUser{} - JWT
// redis - cache coherence + retrieval + save
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

func controllerMiddleware(next http.Handler, db DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uri := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		controllers := []controller{controller_users}
		for _, controller := range controllers {
			for endpointUri, endpointCall := range controller.endpoints {
				if endpointUri == uri {
					// controller.readJson(r, func() {
					// 	return nil
					// })

					cStatus, cData, cErr := endpointCall(db, func(data any) error {
						return controller.readJson(r, data)
					})
					if cErr != nil {
						controller.retErr(w, cErr)
					} else {
						controller.retJson(w, cStatus, cData)
					}
					return
				}
			}
		}
		//

		// http.Error(w, "Not found", http.StatusNotFound)
		next.ServeHTTP(w, r)
	})
}

func main() {
	db := DBNew()
	db.Init()
	mainHandler := http.NewServeMux()
	http.ListenAndServe("0.0.0.0:9090", controllerMiddleware(mainHandler, db))
	db.Close()
}
