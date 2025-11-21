package store

import "context"

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
