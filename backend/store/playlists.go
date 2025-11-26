package store

import (
	"context"
	"log"
	"time"
)

// PlaylistV2 : Same as playlist but with song count
type PlaylistV2 struct {
	ID          int64
	Name        string
	ImageURL    string
	UserID      int64
	Description string
	SourceType  string // spotify', 'osu'
	ExternalID  string // The ID on Spotify/osu!
	CreatedAt   time.Time
	SongCount   int64
}

func (s *Store) SavePlaylist(ctx context.Context, p *Playlist) (int64, error) {
	query := `
	INSERT INTO playlists (user_id, name, description, image_url, source_type, external_id)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id, source_type, external_id) DO UPDATE SET
		name = excluded.name,
		description = excluded.description,
		image_url = excluded.image_url;
	`
	_, err := s.db.ExecContext(ctx, query, p.UserID, p.Name, p.Description, p.ImageURL, p.SourceType, p.ExternalID)
	if err != nil {
		return 0, err
	}

	var id int64
	err = s.db.QueryRowContext(ctx, "SELECT id FROM playlists WHERE user_id=? AND source_type=? AND external_id=?",
		p.UserID, p.SourceType, p.ExternalID).Scan(&id)
	return id, err
}

// AddSongToPlaylist links them in the join table
func (s *Store) AddSongToPlaylist(ctx context.Context, playlistID, songID int64) error {
	query := `INSERT OR IGNORE INTO playlist_songs (playlist_id, song_id) VALUES (?, ?)`
	_, err := s.db.ExecContext(ctx, query, playlistID, songID)
	return err
}

func (s *Store) GetPlaylists() ([]*PlaylistV2, error) {
	query := "SELECT * from playlists"
	playlists, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}

	defer func() {
		err := playlists.Close()
		if err != nil {
			log.Fatalf("Failed to close playlist songs row: %v", err)
			return
		}
	}()

	formatedPlaylist := make([]*PlaylistV2, 0)

	for playlists.Next() {
		ps := &Playlist{} // Initialize the pointer
		var sc int64

		if err := playlists.Scan(&ps.ID, &ps.UserID, &ps.Name, &ps.Description, &ps.ImageURL, &ps.SourceType, &ps.ExternalID, &ps.CreatedAt); err != nil {
			return nil, err
		}

		songCount := s.db.QueryRow("SELECT count(id) from playlist_songs where playlist_id = ?", ps.ID)
		if err := songCount.Scan(&sc); err != nil {
			return nil, err
		}

		formatedPlaylist = append(formatedPlaylist, &PlaylistV2{
			ID:          ps.ID,
			UserID:      ps.UserID,
			Name:        ps.Name,
			Description: ps.Description,
			ImageURL:    ps.ImageURL,
			SourceType:  ps.SourceType,
			ExternalID:  ps.ExternalID,
			CreatedAt:   ps.CreatedAt,
			SongCount:   sc,
		})
	}
	return formatedPlaylist, nil
}
