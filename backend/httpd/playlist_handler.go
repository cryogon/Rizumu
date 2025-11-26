package httpd

import (
	"encoding/json"
	"log"
	"net/http"
)

func (s *Server) getPlaylists() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playlists, err := s.Store.GetPlaylists()
		if err != nil {
			log.Fatalf("Failed to fetch playlists. err: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(playlists)
		if err != nil {
			log.Fatalf("Failed to encode playlists. err: %v", err)
			return
		}
	}
}
