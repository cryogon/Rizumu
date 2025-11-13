package httpd

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"cryogon/rizumu-backend/downloader"
)

func NewRouter(dlSvc *downloader.Service) http.Handler {
	// Create the Server struct that holds our service
	srv := &Server{
		Downloader: dlSvc,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// --- API Routes ---
	r.Post("/download", srv.handleCreateDownload())

	return r
}
