package store

import (
	"context"
	"database/sql"
	"log"
)

type SongConfig struct {
	offset int64
	limit  int64
}

func (s *Store) SaveSong(ctx context.Context, song *Song) (int64, error) {
	query := `
	INSERT INTO songs (title, artist, album, image_url, duration_ms, bpm, energy, valence, provider, provider_id, raw_metadata, status)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(provider, provider_id) DO UPDATE SET
		image_url = excluded.image_url,
		bpm = excluded.bpm,
		energy = excluded.energy,
		valence = excluded.valence,
		raw_metadata = excluded.raw_metadata;
	`
	// Note: We default status to 'Pending' in the struct if not set
	if song.Status == "" {
		song.Status = "Pending"
	}

	_, err := s.db.ExecContext(ctx, query,
		song.Title, song.Artist, song.Album, song.ImageURL, song.DurationMs,
		song.BPM, song.Energy, song.Valence,
		song.Provider, song.ProviderID, song.RawMetadata, song.Status,
	)
	if err != nil {
		return 0, err
	}

	var id int64
	err = s.db.QueryRowContext(ctx, "SELECT id FROM songs WHERE provider=? AND provider_id=?",
		song.Provider, song.ProviderID).Scan(&id)
	return id, err
}

func (s *Store) GetSong(ctx context.Context, id int64) (*Song, error) {
	query := `SELECT id, title, artist, album, image_url, provider, provider_id, file_path, status, bpm, energy, valence, duration_ms 
	          FROM songs WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var song Song
	var filePath sql.NullString
	err := row.Scan(&song.ID, &song.Title, &song.Artist, &song.Album, &song.ImageURL,
		&song.Provider, &song.ProviderID, &filePath, &song.Status, &song.BPM, &song.Energy, &song.Valence, &song.DurationMs)
	if err != nil {
		return nil, err
	}
	song.FilePath = filePath.String
	return &song, nil
}

func (s *Store) GetSongs(ctx context.Context, config SongConfig) ([]*Song, error) {
	query := `
	SELECT id, title, artist, album, image_url, provider, provider_id, file_path, status, bpm, energy, valence, duration_ms FROM songs
	where id > ?
  LIMIT ?
	`
	rows, err := s.db.QueryContext(ctx, query, config.offset, config.limit)
	if err != nil {
		return nil, err
	}

	var songs []*Song

	for rows.Next() {
		var song Song
		var filePath sql.NullString
		err := rows.Scan(&song.ID, &song.Title, &song.Artist, &song.Album, &song.ImageURL,
			&song.Provider, &song.ProviderID, &filePath, &song.Status, &song.BPM, &song.Energy, &song.Valence, &song.DurationMs)
		if err != nil {
			return nil, err
		}
		song.FilePath = filePath.String
		songs = append(songs, &song)
	}

	return songs, nil
}

func (s *Store) DeleteSong(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM playlist_songs WHERE song_id = ?", id); err != nil {
		tx.Rollback()
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM play_history WHERE song_id = ?", id); err != nil {
		tx.Rollback()
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM song_tags WHERE song_id = ?", id); err != nil {
		tx.Rollback()
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM songs WHERE id = ?", id); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (s *Store) UpdateSongStatus(ctx context.Context, songID int64, status string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE songs SET status = ? WHERE id = ?", status, songID)
	return err
}

func (s *Store) UpdateSongPath(ctx context.Context, songID int64, path string, fileSize int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE songs SET file_path = ?, file_size = ?, status = 'Downloaded' WHERE id = ?", path, fileSize, songID)
	return err
}

func (s *Store) UpdateSongProgress(ctx context.Context, id int64, progress float64) error {
	// Optional: Add a 'progress' column if you want persistence, or just mark Downloading
	_, err := s.db.ExecContext(ctx, "UPDATE songs SET status = 'Downloading' WHERE id = ?", id)
	return err
}

func (s *Store) UpdateSongFullMetadata(ctx context.Context, song *Song) error {
	query := `
	UPDATE songs 
	SET file_path = ?, image_url = ?, title = ?, artist = ?, bpm = ?, duration_ms = ?, status = 'Downloaded'
	WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, song.FilePath, song.ImageURL, song.Title, song.Artist, song.BPM, song.ID, song.DurationMs)
	return err
}

func (s *Store) ResetStuckDownloads(ctx context.Context) error {
	query := `UPDATE songs SET status = 'Pending' WHERE status = 'Downloading'`
	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		log.Printf("Reset %d rows", rows)
	}
	return nil
}
