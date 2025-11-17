package store

import "log"

func (s *Store) migrate() error {
	query := `
    CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        username TEXT NOT NULL, 
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    -- 2. The External Connections (Spotify, Osu, YTM)
    CREATE TABLE IF NOT EXISTS connections (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER NOT NULL,
        
        provider TEXT NOT NULL,     -- e.g., 'spotify', 'osu', 'ytmusic'
        provider_id TEXT NOT NULL,  -- e.g., 'cryogon', '1029384'
        
        access_token TEXT,          -- OAuth access token
        refresh_token TEXT,         -- OAuth refresh token (or NULL for YTM)
        expiry DATETIME,            -- When the token dies
        
        -- For YTM, we store the cookie string here. 
        -- For others, maybe extra JSON config.
        metadata TEXT,              

        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        
        FOREIGN KEY(user_id) REFERENCES users(id),
        UNIQUE(provider, provider_id) -- Prevent duplicate links
    );
    `

	_, err := s.db.Exec(query)
	if err != nil {
		log.Printf("ERROR: Database migration failed: %v", err)
		return err
	}

	return nil
}
