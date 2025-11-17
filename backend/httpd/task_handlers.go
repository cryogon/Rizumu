package httpd

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"cryogon/rizumu-backend/downloader"

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

		task, err := s.Downloader.CreateDownload(req)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		respondWithJSON(w, http.StatusAccepted, task) // 202 Accepted is perfect
	}
}

func (s *Server) handleGetTaskStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "taskID")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid task ID", http.StatusBadRequest)
			return
		}

		task, err := s.Downloader.GetTaskStatus(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		respondWithJSON(w, http.StatusOK, task)
	}
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
