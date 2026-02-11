package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"mvpapp/internal/web"
)

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/app.db"
	}

	app, err := web.NewAppWithSQLite(dbPath)
	if err != nil {
		log.Fatalf("failed to initialize database at %s: %v", dbPath, err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	baseURL := os.Getenv("DASHBOARD_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", port)
	}
	app.SetDashboardURL(baseURL)

	addr := ":" + port
	log.Printf("starting server on %s", addr)
	if err := http.ListenAndServe(addr, app.Handler()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
