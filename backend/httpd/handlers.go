package httpd

import (
	"encoding/json"
	"log"
	"net/http"

	"cryogon/rizumu-backend/downloader"
)

type Server struct {
	Downloader *downloader.Service
}

func (s *Server) handleCreateDownload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req downloader.DownloadPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: decoding request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		task, err := s.Downloader.CreateDownload(req)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		respondWithJSON(w, http.StatusAccepted, task) // 202 Accepted is perfect
	}
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if payload != nil {
		json.NewEncoder(w).Encode(payload)
	}
}
