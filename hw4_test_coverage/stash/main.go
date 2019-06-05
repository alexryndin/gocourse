package main

import (
	"net/http"
	"fmt"
	"time"
)

func unknHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Print(w.Header().Get("Location"))
	w.Header().Del("Location")
	w.Header().Set("Location", "")
	w.WriteHeader(http.StatusOK)
	w.Header().Write(w)
	return
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", unknHandler)
	server := http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	fmt.Println("starting server at :8080")
	server.ListenAndServe()
}
