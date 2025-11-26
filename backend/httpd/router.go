// Package httpd handles api
package httpd

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"cryogon/rizumu-backend/downloader"
	"cryogon/rizumu-backend/spotify"
	"cryogon/rizumu-backend/store"
)

type Server struct {
	Downloader *downloader.Service
	Spotify    *spotify.Client
	Store      *store.Store
}

func NewRouter(dlSvc *downloader.Service, spotifyClient *spotify.Client, db *store.Store) http.Handler {
	srv := &Server{
		Downloader: dlSvc,
		Spotify:    spotifyClient,
		Store:      db,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Task Routes (from task_handlers.go)
	r.Post("/download", srv.handleCreateDownload())
	r.Get("/tasks/{taskID}", srv.handleGetTaskStatus())
	r.Get("/stream/{songID}", srv.handleStreamSong())

	// Auth Routes (from auth_handlers.go)
	r.Get("/auth/spotify/login", srv.handleSpotifyLogin())
	r.Get("/auth/spotify/callback", srv.handleSpotifyCallback())
	r.Post("/me/sync", srv.handleSyncSpotify())

	// Playlists
	r.Get("/playlists", srv.getPlaylists())

	return r
}
