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

    CREATE TABLE IF NOT EXISTS playlists (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER NOT NULL,
        name TEXT NOT NULL,
        description TEXT,
        image_url TEXT,
        
        -- 'rizumu' = Created by you. 'spotify' = Synced.
        source_type TEXT DEFAULT 'rizumu', 
        
        -- If synced, this is the Spotify/YTM ID. If custom, this is NULL.
        external_id TEXT, 
        
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(user_id, source_type, external_id)
    );

    CREATE TABLE IF NOT EXISTS songs (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        
        -- Basic Info
        title TEXT NOT NULL,
        artist TEXT NOT NULL,
        album TEXT,
				image_url TEXT,
	      lyrics TEXT,
        duration_ms INTEGER,
        
        -- Audio Analysis (For Recommendation Algo! if needed)
        bpm REAL,                  -- Crucial for osu!
        key TEXT,                  -- Useful for mixing
        energy REAL,               -- 0.0 to 1.0 (Spotify gives this)
        valence REAL,              -- 0.0 to 1.0 (Mood: Happy/Sad)

				-- Usage Stats
				play_count INTEGER DEFAULT 0,  -- <--- NEW: Increments on play
        last_played_at DATETIME,       -- <--- NEW: For "Recently Played"
        is_favorite BOOLEAN DEFAULT 0, -- <--- NEW: Simple "Like" button
        
        -- Source Info
        provider TEXT NOT NULL,    -- 'spotify', 'osu', 'ytmusic', 'local'
        provider_id TEXT NOT NULL, -- The ID on that platform
        
        -- Files
        file_path TEXT,
        file_size INTEGER DEFAULT 0,   -- <--- NEW: Bytes
        bitrate INTEGER DEFAULT 0,     -- <--- NEW: e.g. 320
        format TEXT,                   -- <--- NEW: 'mp3', 'ogg'            -- Path on disk (e.g., "/songs/123.mp3")
        
        -- Raw Data
        -- We dump the WHOLE JSON from Spotify/YTM here.
        raw_metadata TEXT,         
        
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(provider, provider_id)
    );

    -- Playlist Entries (The Join Table)
    CREATE TABLE IF NOT EXISTS playlist_songs (
        id INTEGER PRIMARY KEY AUTOINCREMENT, -- Added ID for ordering
        playlist_id INTEGER NOT NULL,
        song_id INTEGER NOT NULL,
        
        -- Allows you to reorder custom playlists
        sort_order INTEGER DEFAULT 0, 
        
        added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY(playlist_id) REFERENCES playlists(id),
        FOREIGN KEY(song_id) REFERENCES songs(id)
    );

    -- Tags/Genres 
    -- We normalize tags so "Pop" is stored once, but linked to 1000 songs.
    CREATE TABLE IF NOT EXISTS tags (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT UNIQUE NOT NULL -- 'pop', 'anime', 'high-bpm', 'rock'
    );

    -- Song <-> Tag Link
    CREATE TABLE IF NOT EXISTS song_tags (
        song_id INTEGER NOT NULL,
        tag_id INTEGER NOT NULL,
        PRIMARY KEY (song_id, tag_id),
        FOREIGN KEY(song_id) REFERENCES songs(id),
        FOREIGN KEY(tag_id) REFERENCES tags(id)
    );

	  CREATE TABLE IF NOT EXISTS play_history (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER NOT NULL,
        song_id INTEGER NOT NULL,
        played_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        duration_listened_ms INTEGER, -- Did they skip halfway?
        
        FOREIGN KEY(user_id) REFERENCES users(id),
        FOREIGN KEY(song_id) REFERENCES songs(id)
    );
    `

	_, err := s.db.Exec(query)
	if err != nil {
		log.Printf("ERROR: Database migration failed: %v", err)
		return err
	}

	return nil
}
