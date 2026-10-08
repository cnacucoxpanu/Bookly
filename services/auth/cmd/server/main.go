
package main

import (
	"log"
	"net/http"

	"bookly/services/auth/internal/handler"
)

func main() {
	healthHandler := handler.NewHealthHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler.Check)

	server := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}

	log.Println("Auth Service started on :8081")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}