package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"cryogon/rizumu-backend/downloader"
	"cryogon/rizumu-backend/httpd"
	"cryogon/rizumu-backend/ipc"
	"cryogon/rizumu-backend/player"
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

	if err := db.ResetStuckDownloads(context.Background()); err != nil {
		log.Printf("WARN: Failed to reset stuck downloads: %v", err)
	}

	dlSvc := downloader.NewService(db)
	spotifyClient := spotify.NewClient(spotifyClientID, spotifyClientSecret)

	musicPlayer := player.NewPlayer(dlSvc, db)

	ipcHandler := ipc.NewIPCHandler(musicPlayer, *db)

	// start ipc server on different thread
	go ipcHandler.Init()

	router := httpd.NewRouter(dlSvc, spotifyClient, db, musicPlayer)

	log.Println("Server listening on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
