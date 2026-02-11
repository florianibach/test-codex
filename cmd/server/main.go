package main

import (
	"log"
	"net/http"
	"os"

	"mvpapp/internal/web"
)

func main() {
	app := web.NewApp()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.Printf("starting server on %s", addr)
	if err := http.ListenAndServe(addr, app.Handler()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
