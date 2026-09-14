package main

import (
	"encoding/json"
	"net/http"
	"time"
)

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

	http.ListenAndServe("0.0.0.0:9090", server)

	time.Sleep(time.Minute * 60)
}
