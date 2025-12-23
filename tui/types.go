package main

import (
	"encoding/json"
	"time"
)

type CommandType string

const (
	CmdPlay      CommandType = "play"
	CmdPause     CommandType = "pause"
	CmdStop      CommandType = "stop"
	CmdDownload  CommandType = "download"
	CmdNext      CommandType = "next"
	CmdPrev      CommandType = "prev"
	CmdSongs     CommandType = "songs" // returns song
	CmdPlaylists CommandType = "playlists"
)

type Message struct {
	Type string          `json:"type"` // "cmd", "pstate"
	Data json.RawMessage `json:"data"`
}

type Command struct {
	Type       CommandType `json:"type"`
	SongID     int64       `json:"song_id"`
	PlaylistID int64       `json:"playlist_id"`
}

type PlayerState struct {
	Playing  bool   `json:"playing"`
	SongID   int64  `json:"song_id"`
	SongName string `json:"song_name"`
	Artist   string `json:"artist"`
	Progress int    `json:"progreass"` // Current Song Pos
	Duration int    `json:"duration"`  // Song's Duration
}

type Playlist struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ImageURL    string    `json:"image_url"`
	SourceType  string    `json:"source_type"` // 'rizumu', 'spotify', 'osu'
	ExternalID  string    `json:"external_id"` // The ID on Spotify/osu!
	CreatedAt   time.Time `json:"created_at"`
}

// Implement list.Item interface for Playlist so we can use it in bubbles/list
func (p Playlist) FilterValue() string { return p.Name }
func (p Playlist) Title() string       { return p.Name }

// Song is the master record for a track
type Song struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	ImageURL   string `json:"image_url"`
	Lyrics     string `json:"lyrics,omitempty"`
	DurationMs int64  `json:"duration_ms"`

	// Audio Analysis (For your Algo)
	BPM     float64 `json:"bpm"`
	Energy  float64 `json:"energy"`
	Valence float64 `json:"valence"`

	// Usage Stats
	PlayCount    int64      `json:"play_count"`
	LastPlayedAt *time.Time `json:"last_played_at"` // Pointer allows null (never played)
	IsFavorite   bool       `json:"is_favorite"`

	// Source Info
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id"`

	// File Info
	FilePath string `json:"file_path,omitempty"` // Empty if not downloaded
	FileSize int64  `json:"file_size,omitempty"`
	Bitrate  int    `json:"bitrate,omitempty"`
	Format   string `json:"format,omitempty"`

	// Raw Data (Hidden from JSON usually, but useful for debugging)
	RawMetadata string `json:"-"`

	CreatedAt time.Time `json:"created_at"`

	// State
	Status string `json:"status"`
}
