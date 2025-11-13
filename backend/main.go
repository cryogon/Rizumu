package main

import (
	"log"
	"net/http"

	// Your project's packages
	"cryogon/rizumu-backend/downloader"
	"cryogon/rizumu-backend/httpd"
)

func main() {
	log.Println("Starting Rizumu Backend...")

	// 1. Create the Downloader service (which starts the worker)
	dlSvc := downloader.NewService()

	// 2. Create the HTTP router (and give it the service)
	router := httpd.NewRouter(dlSvc)

	// 3. Start the server and BLOCK forever
	log.Println("Server listening on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
