package main

import (
	"log"
	"net/http"
	"os"

	"mystic-square/internal/app"
)

// @title			Mystic Square API
// @version		1.0
// @description	HTTP API for managing Mystic Square puzzle levels.
// @schemes		http
// @host			localhost:8080
// @BasePath		/api/v1
func main() {
	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, app.NewRouter()); err != nil {
		log.Fatal(err)
	}
}
