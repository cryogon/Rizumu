package main

import (
	"log"
	"net/http"

	"cryogon/rizumu-backend/downloader"
	"cryogon/rizumu-backend/httpd"
)

func main() {
	log.Println("Starting Rizumu Backend...")

	dlSvc := downloader.NewService()

	router := httpd.NewRouter(dlSvc)

	log.Println("Server listening on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
