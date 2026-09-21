package main

import (
	"encoding/json"
	"net/http"
	"time"
)

var userRepo = NewUserRepo()

var orderRepo = NewOrderRepo()

func main() {

	server := http.NewServeMux()

	server.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		responseData := struct{ APIVersion string }{APIVersion: "1.0"}

		responseBytes, responseBytesErr := json.Marshal(responseData)
		r.URL.Query().Get("asd")
		if responseBytesErr != nil {
			panic(responseBytesErr)
		}

		_, writeErr := w.Write(responseBytes)
		if writeErr != nil {
			panic(writeErr)
		}
	})

	server.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		// userList := userRepo.findAll(5, 0)

		user := userRepo.findByName(func(arg User) bool {
			return arg.Name == "Vazgen"
		})
		responseBytes, responseBytesErr := json.Marshal(user)

		if responseBytesErr != nil {
			panic(responseBytesErr)
		}

		_, writeErr := w.Write(responseBytes)
		if writeErr != nil {
			panic(writeErr)
		}
	})

	server.HandleFunc("GET /orders", func(w http.ResponseWriter, r *http.Request) {
		// orderList := orderRepo.findAll(5, 0)

		order := orderRepo.findByName(func(arg Order) bool {
			return arg.Price == 10
		})

		responseBytes, responseBytesErr := json.Marshal(order)

		if responseBytesErr != nil {
			panic(responseBytesErr)
		}

		_, writeErr := w.Write(responseBytes)
		if writeErr != nil {
			panic(writeErr)
		}
	})

	http.ListenAndServe("0.0.0.0:9090", server)

	time.Sleep(time.Minute * 60)
}
