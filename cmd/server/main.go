package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"mvpapp/internal/web"
)

func main() {
	app := web.NewApp()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	baseURL := os.Getenv("DASHBOARD_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", port)
	}
	app.SetDashboardURL(baseURL)
	app.StartBackgroundPromotion(30 * time.Second)

	addr := ":" + port
	log.Printf("starting server on %s", addr)
	if err := http.ListenAndServe(addr, app.Handler()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
