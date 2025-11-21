package store

import (
	"time"
)

// User represents the human owner of the account (You)
type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

// Connection represents a link to an external service (Spotify, osu!, YTM)
type Connection struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Provider     string    `json:"provider"`    // 'spotify', 'osu!', 'ytmusic'
	ProviderID   string    `json:"provider_id"` // The ID on that platform
	AccessToken  string    `json:"-"`           // Never send to client
	RefreshToken string    `json:"-"`           // Never send to client
	Expiry       time.Time `json:"expiry"`
	Metadata     string    `json:"-"` // Cookies or extra config
	CreatedAt    time.Time `json:"created_at"`
}

// Playlist is a collection of songs (Synced or Custom)
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

// PlaylistSong links a Song to a Playlist
type PlaylistSong struct {
	ID         int64     `json:"id"`
	PlaylistID int64     `json:"playlist_id"`
	SongID     int64     `json:"song_id"`
	SortOrder  int       `json:"sort_order"`
	AddedAt    time.Time `json:"added_at"`
}

// PlayLog records history for your algorithm
type PlayLog struct {
	ID                 int64     `json:"id"`
	UserID             int64     `json:"user_id"`
	SongID             int64     `json:"song_id"`
	PlayedAt           time.Time `json:"played_at"`
	DurationListenedMs int64     `json:"duration_listened_ms"`
}

// Tag allows for genres like "Pop", "High BPM", "Anime"
type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
