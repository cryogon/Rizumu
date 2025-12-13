package httpd

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"cryogon/rizumu-backend/downloader"
	"cryogon/rizumu-backend/utils"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleStreamSong() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "songID")
		songID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid song ID", 400)
			return
		}

		song, err := s.Store.GetSong(r.Context(), songID)
		if err != nil {
			http.Error(w, "Song not found", 404)
			return
		}

		if song.Status == "Not Available" {
			http.Error(w, "Can't download this song", 404)
			return
		}

		fileReady := false
		if song.FilePath != "" {
			if _, err := os.Stat(song.FilePath); err == nil {
				fileReady = true
			}
		}

		if fileReady {
			log.Printf("Streaming Song: %s", song.Title)
			http.ServeFile(w, r, song.FilePath)
			return
		}

		if song.Status == string(downloader.StatusDownloading) {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("Buffering..."))
			return
		}

		log.Printf("Song %d not found on disk. Trigerring new download.", songID)
		payload := downloader.DownloadPayload{
			Mode: "download",
			URL:  "",
		}

		sourceURL, err := utils.GetSourceURL(song.Provider, song.ProviderID)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		payload.URL = sourceURL

		_, err = s.Downloader.CreateDownload(payload, songID)
		if err != nil {
			log.Printf("Fauled to auto-download: %v", err)
			http.Error(w, "Failed to start download", 500)
			return
		}

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Buffering..."))
	}
}
