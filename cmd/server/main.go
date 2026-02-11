package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"mvpapp/internal/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/app.db"
	}

	app, err := web.NewAppWithSQLite(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database at %s: %w", dbPath, err)
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
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}
