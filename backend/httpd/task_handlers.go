package httpd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"cryogon/rizumu-backend/downloader"
	"cryogon/rizumu-backend/store"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleCreateDownload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req downloader.DownloadPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: decoding request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		source, err := s.Downloader.GetSource(req)
		if err != nil {
			log.Printf("ERROR: bad source found")
			http.Error(w, err.Error(), 400)
			return
		}

		newSong := &store.Song{
			Title:      "Unknown (Pending)",
			Artist:     "Unknown",
			Provider:   source.String(),
			ProviderID: req.URL,
			Status:     "Pending",
		}

		dbID, err := s.Store.SaveSong(r.Context(), newSong)
		if err != nil {
			http.Error(w, "Failed to create DB entry", 500)
			return
		}

		task, err := s.Downloader.CreateDownload(req, dbID)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		respondWithJSON(w, http.StatusAccepted, task) // 202 Accepted is perfect
	}
}

func (s *Server) handleGetTaskStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "taskID"), 10, 64)
		song, err := s.Store.GetSong(r.Context(), id)
		if err != nil {
			http.Error(w, "Not found", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		respondWithJSON(w, 200, song)
	}
}

func (s *Server) processPlaylistDownload(w http.ResponseWriter, r *http.Request, url string) {
	conn, _ := s.Store.GetSpotifyConnection(r.Context(), 1)
	if conn == nil {
		http.Error(w, "Login required", 401)
		return
	}

	songs, err := s.Spotify.FetchTracksFromURL(r.Context(), conn.ToOAuthToken(), url)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	count := 0
	for _, song := range songs {
		dbID, _ := s.Store.SaveSong(r.Context(), song)
		payload := downloader.DownloadPayload{Mode: "download", URL: "https://open.spotify.com/track/" + song.ProviderID}
		if _, err := s.Downloader.CreateDownload(payload, dbID); err == nil {
			count++
		}
	}
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, `{"queued": %d}`, count)
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if payload == nil {
		return
	}
	err := json.NewEncoder(w).Encode(payload)
	if err != nil {
		log.Printf("ERROR: Failed to encode JSON response: %v", err)
	}
}
