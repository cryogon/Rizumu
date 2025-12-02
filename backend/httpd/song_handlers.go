package httpd

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"cryogon/rizumu-backend/store"

	"github.com/go-chi/chi/v5"
)

func (s *Server) getSongs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		songs, err := s.Store.GetSongs(r.Context(), store.SongConfig{})
		if err != nil {
			log.Fatalf("Failed to fetch songs. err: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		err = json.NewEncoder(w).Encode(songs)
		if err != nil {
			log.Fatalf("Failed to encode songs. err: %v", err)
			return
		}
	}
}

func (s *Server) deleteSong() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		songID, err := strconv.ParseInt(chi.URLParam(r, "songID"), 10, 64)
		if err != nil {
			log.Printf("Invalid song ID. err:%v", err)
			http.Error(w, "Invalid song ID", 400)
			return
		}

		song, err := s.Store.GetSong(r.Context(), songID)
		if err != nil {
			http.Error(w, "Song not found", 404)
			return
		}

		if song.FilePath != "" {
			if err := os.Remove(song.FilePath); err != nil && !os.IsNotExist(err) {
				log.Printf("WARN: Failed to delete audio file %s: %v", song.FilePath, err)
			}
		}

		if song.ImageURL != "" && !strings.HasPrefix(song.ImageURL, "http") {
			if err := os.Remove(song.ImageURL); err != nil && !os.IsNotExist(err) {
				log.Printf("WARN: Failed to delete cover image %s: %v", song.ImageURL, err)
			}
		}

		err = s.Store.DeleteSong(r.Context(), songID)
		if err != nil {
			log.Printf("Failed to delete the song with id: %d", songID)
			http.Error(w, "Failed to delete song record", 500)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Song deleted successfully"))
	}
}
