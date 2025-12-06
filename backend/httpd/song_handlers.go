package httpd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"cryogon/rizumu-backend/store"

	"github.com/go-chi/chi/v5"
)

type ApiSong struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	ImageURL string `json:"image_url"`
	Lyrics   string `json:"lyrics,omitempty"`
	Duration string `json:"duration"` // duration

	// Audio Analysis (For your Algo)
	BPM     float64 `json:"bpm"`
	Energy  float64 `json:"energy"`
	Valence float64 `json:"valence"`

	// Usage Stats
	PlayCount    int64      `json:"play_count"`     // play_count
	LastPlayedAt *time.Time `json:"last_played_at"` // last_played_at
	IsFavorite   bool       `json:"is_favorite"`    // is_favorite

	// Source Info
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id"`

	// File Info
	FilePath string `json:"file_path,omitempty"` // file_path
	FileSize int64  `json:"file_size,omitempty"` // file_size
	Bitrate  int    `json:"bitrate,omitempty"`
	Format   string `json:"format,omitempty"`

	// Raw Data (Hidden from JSON usually, but useful for debugging)
	RawMetadata string `json:"-"` // Ignored by JSON marshaler

	CreatedAt time.Time `json:"created_at"`

	// State
	Status string `json:"status"`
}

func (s *Server) getSongs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		var offset int64 = 0
		var limit int64 = 10

		if query.Has("offset") {
			o, e := strconv.ParseInt(query.Get("offset"), 10, 64)
			if e != nil {
				return
			}
			offset = o
		}

		if query.Has("limit") {
			l, e := strconv.ParseInt(query.Get("limit"), 10, 64)
			if e != nil {
				return
			}
			limit = l
		}

		songs, err := s.Store.GetSongs(r.Context(), store.SongConfig{
			Offset: offset,
			Limit:  limit,
		})
		if err != nil {
			log.Fatalf("Failed to fetch songs. err: %v", err)
			return
		}

		apiSongs := make([]ApiSong, 0, len(songs))
		for _, song := range songs {
			apiSongs = append(apiSongs, toAPISong(song))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		err = json.NewEncoder(w).Encode(apiSongs)
		if err != nil {
			log.Fatalf("Failed to encode songs. err: %v", err)
			return
		}
	}
}

func (s *Server) getSongsByPlaylist() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playlistID, err := strconv.ParseInt(chi.URLParam(r, "playlistID"), 10, 64)
		if err != nil {
			log.Printf("Invalid playlist ID. err:%v", err)
			http.Error(w, "Invalid playlist ID", 400)
			return
		}

		songs, err := s.Store.GetSongsByPlaylist(r.Context(), playlistID)
		if err != nil {
			log.Fatalf("Failed to fetch songs. err: %v", err)
			return
		}

		apiSongs := make([]ApiSong, 0, len(songs))
		for _, song := range songs {
			apiSongs = append(apiSongs, toAPISong(song))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		err = json.NewEncoder(w).Encode(apiSongs)
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

func (s *Server) playSong() http.HandlerFunc {
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

		// clear current playing song first
		s.player.Close()

		s.player.AddSong(*song)
		err = s.player.LoadSong(0)
		if err != nil {
			log.Fatalf("Failed to play song: %v", err)
			return
		}

		fmt.Printf("Playlist: %v\n", s.player.GetPlaylist())
		s.player.Play()
		fmt.Printf("Playing: %s\n", s.player.GetCurrentSong().Title)

		respondWithJSON(w, 200, map[string]string{"msg": "Playing"})
	}
}

func (s *Server) pauseResume() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := s.player.TogglePause()
		respondWithJSON(w, 200, map[string]string{"state": state})
	}
}

func (s *Server) stopSong() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.player.Close()
		respondWithJSON(w, 200, map[string]string{"state": "Stopped"})
	}
}

// Helpers
func formatDuration(ms int64) string {
	seconds := ms / 1000
	minutes := seconds / 60
	remainingSeconds := seconds % 60
	return fmt.Sprintf("%d:%02d", minutes, remainingSeconds)
}

func toAPISong(s *store.Song) ApiSong {
	// Custom value formatting logic
	durationStr := formatDuration(s.DurationMs)

	return ApiSong{
		ID:           s.ID,
		Title:        s.Title,
		Artist:       s.Artist,
		Album:        s.Album,
		ImageURL:     s.ImageURL,
		Lyrics:       s.Lyrics,
		Duration:     durationStr,
		BPM:          s.BPM,
		Energy:       s.Energy,
		Valence:      s.Valence,
		PlayCount:    s.PlayCount,
		LastPlayedAt: s.LastPlayedAt,
		IsFavorite:   s.IsFavorite,
		Provider:     s.Provider,
		ProviderID:   s.ProviderID,
		FilePath:     s.FilePath,
		FileSize:     s.FileSize,
		Bitrate:      s.Bitrate,
		Format:       s.Format,
		CreatedAt:    s.CreatedAt,
		Status:       s.Status,
	}
}
