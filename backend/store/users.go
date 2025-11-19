package store

import (
	"context"
	"database/sql"

	"golang.org/x/oauth2"
)

// SaveSpotifyConnection links a Spotify account to a User
func (s *Store) SaveSpotifyConnection(ctx context.Context, userID int64, spotifyID string, token *oauth2.Token) error {
	// Upsert: Insert or Update if it exists
	query := `
	INSERT INTO connections (user_id, provider, provider_id, access_token, refresh_token, expiry)
	VALUES (?, 'spotify', ?, ?, ?, ?)
	ON CONFLICT(provider, provider_id) DO UPDATE SET
		access_token = excluded.access_token,
		refresh_token = excluded.refresh_token,
		expiry = excluded.expiry;
	`

	_, err := s.db.ExecContext(ctx, query,
		userID,
		spotifyID,
		token.AccessToken,
		token.RefreshToken,
		token.Expiry,
	)

	return err
}

// GetSpotifyConnection retrieves the token for a specific user
func (s *Store) GetSpotifyConnection(ctx context.Context, userID int64) (*Connection, error) {
	query := `
    SELECT id, user_id, provider, provider_id, access_token, refresh_token, expiry 
    FROM connections 
    WHERE user_id = ? AND provider = 'spotify'
    LIMIT 1`

	row := s.db.QueryRowContext(ctx, query, userID)

	var c Connection
	err := row.Scan(&c.ID, &c.UserID, &c.Provider, &c.ProviderID, &c.AccessToken, &c.RefreshToken, &c.Expiry)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &c, nil
}

// CreateAdminUser Helper to create the initial "Admin" user if your DB is empty
func (s *Store) CreateAdminUser(ctx context.Context) (int64, error) {
	// Check if user exists
	var id int64
	err := s.db.QueryRowContext(ctx, "SELECT id FROM users LIMIT 1").Scan(&id)
	if err == nil {
		return id, nil // User already exists
	}

	// Create default user
	res, err := s.db.ExecContext(ctx, "INSERT INTO users (username) VALUES (?)", "admin")
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (c *Connection) ToOAuthToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  c.AccessToken,
		RefreshToken: c.RefreshToken,
		Expiry:       c.Expiry,
		TokenType:    "Bearer",
	}
}
