package main

import (
	"log"
	"net/http"
	"os"

	"cryogon/rizumu-backend/downloader"
	"cryogon/rizumu-backend/httpd"
	"cryogon/rizumu-backend/spotify"
	"cryogon/rizumu-backend/store"

	"github.com/joho/godotenv"
)

func main() {
	log.Println("Starting Rizumu Backend...")

	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	spotifyClientID := os.Getenv("SPOTIFY_CLIENT_ID")
	spotifyClientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if spotifyClientID == "" || spotifyClientSecret == "" {
		log.Fatal("FATAL: SPOTIFY_ID and SPOTIFY_SECRET must be set")
	}

	db, err := store.NewSQLiteStore("rizumu.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	dlSvc := downloader.NewService()
	spotifyClient := spotify.NewClient(spotifyClientID, spotifyClientSecret)

	// 2. Create the HTTP router (and give it the service)
	router := httpd.NewRouter(dlSvc, spotifyClient, db)

	// 3. Start the server and BLOCK forever
	log.Println("Server listening on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
