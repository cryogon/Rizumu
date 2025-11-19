package spotify

import (
	"context"
	"encoding/json"
	"log"

	"cryogon/rizumu-backend/store"

	"github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2"
)

// SyncAll fetches everything and saves it to the DB
func (c *Client) SyncAll(ctx context.Context, token *oauth2.Token, db *store.Store, userID int64) error {
	client := c.NewClientFromToken(token)

	// Fetch All User Playlists (Pagination loop)
	playlists, err := client.CurrentUsersPlaylists(ctx)
	if err != nil {
		return err
	}

	for {
		for _, p := range playlists.Playlists {
			log.Printf("Syncing Playlist: %s", p.Name)

			dbPlaylist := &store.Playlist{
				UserID:     userID,
				Name:       p.Name,
				SourceType: "spotify",
				ExternalID: string(p.ID),
			}
			if len(p.Images) > 0 {
				dbPlaylist.ImageURL = p.Images[0].URL
			}

			pID, err := db.SavePlaylist(ctx, dbPlaylist)
			if err != nil {
				log.Printf("Error saving playlist %s: %v", p.Name, err)
				continue
			}

			if err := c.syncPlaylistTracks(ctx, client, db, p.ID, pID); err != nil {
				log.Printf("Error syncing tracks for %s: %v", p.Name, err)
			}
		}

		if err := client.NextPage(ctx, playlists); err != nil {
			if err == spotify.ErrNoMorePages {
				break
			}
			return err
		}
	}
	return nil
}

// syncPlaylistTracks fetches all songs in a playlist
func (c *Client) syncPlaylistTracks(ctx context.Context, client *spotify.Client, db *store.Store, spotifyPlaylistID spotify.ID, dbPlaylistID int64) error {
	// Get tracks (first page)
	tracks, err := client.GetPlaylistItems(ctx, spotifyPlaylistID)
	if err != nil {
		return err
	}

	// We need 3 buckets for our batching logic:
	var idsForAudio []spotify.ID                 // IDs to send to Spotify API
	songsMap := make(map[spotify.ID]*store.Song) // Map to link IDs back to Song objects
	var songsToSave []*store.Song                // The final list to save to DB (includes locals)

	// Helper to process the current batch
	processBatch := func() {
		if len(songsToSave) == 0 {
			return
		}

		// 1. Fetch Audio Features only for valid Spotify IDs
		if len(idsForAudio) > 0 {
			features, err := client.GetAudioFeatures(ctx, idsForAudio...)
			if err != nil {
				// WARN but don't stop. We just won't have BPM for this batch.
				log.Printf("WARN: Failed to get audio features for batch of %d. Error: %v", len(idsForAudio), err)
			} else {
				// 2. Map features back to the songs
				for _, f := range features {
					if f == nil {
						continue
					}

					if song, exists := songsMap[f.ID]; exists {
						song.BPM = float64(f.Tempo)
						song.Energy = float64(f.Energy)
						song.Valence = float64(f.Valence)
					}
				}
			}
		}

		// 3. Save ALL songs to DB (including local ones that we skipped analysis for)
		for _, song := range songsToSave {
			sID, err := db.SaveSong(ctx, song)
			if err != nil {
				log.Printf("Error saving song '%s': %v", song.Title, err)
				continue
			}
			db.AddSongToPlaylist(ctx, dbPlaylistID, sID)
		}

		// 4. Clear buckets for next batch
		idsForAudio = nil
		songsToSave = nil
		songsMap = make(map[spotify.ID]*store.Song)
	}

	for {
		for _, item := range tracks.Items {
			// Skip empty items or Podcasts (Track == nil)
			if item.Track.Track == nil {
				continue
			}

			fullTrack := item.Track.Track

			rawJSON, _ := json.Marshal(fullTrack)

			isAnalysisCandidate := true
			if item.IsLocal || fullTrack.ID == "" || fullTrack.Type != "track" {
				isAnalysisCandidate = false
			}

			provider := "spotify"
			providerID := string(fullTrack.ID)

			if item.IsLocal {
				provider = "local"
				providerID = string(fullTrack.URI)
			}

			// 3. Build Song Struct
			// (Note: We save ALL of them, even podcasts/local files)
			s := &store.Song{
				Title:       fullTrack.Name,
				Artist:      "Unknown Artist",
				Album:       fullTrack.Album.Name,
				DurationMs:  int64(fullTrack.Duration),
				Provider:    provider,
				ProviderID:  providerID,
				RawMetadata: string(rawJSON),
			}

			if len(fullTrack.Artists) > 0 {
				s.Artist = fullTrack.Artists[0].Name
			}
			if len(fullTrack.Album.Images) > 0 {
				s.ImageURL = fullTrack.Album.Images[0].URL
			}

			//  Add to "Save List" (We save everything!)
			songsToSave = append(songsToSave, s)

			// Add to "Analysis List" (ONLY if it's a valid music track)
			if isAnalysisCandidate {
				idsForAudio = append(idsForAudio, fullTrack.ID)
				songsMap[fullTrack.ID] = s
			}

			if len(songsToSave) >= 50 {
				processBatch()
			}
		}

		// Next page of tracks
		if err := client.NextPage(ctx, tracks); err != nil {
			if err == spotify.ErrNoMorePages {
				break
			}
			return err
		}
	}

	// Process final partial batch
	processBatch()
	return nil
}
