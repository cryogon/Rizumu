package httpd

import (
	"log"
	"net/http"

	"golang.org/x/oauth2"
	oauthSpotify "golang.org/x/oauth2/spotify" // Alias this to avoid collision
)

func (s *Server) handleSyncSpotify() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Hardcoded User ID for now (Admin)
		userID := int64(1)

		// 2. Get Connection from DB
		conn, err := s.Store.GetSpotifyConnection(r.Context(), userID)
		if err != nil {
			http.Error(w, "Database error", 500)
			return
		}
		if conn == nil {
			http.Error(w, "No Spotify connection found. Please login first.", 400)
			return
		}

		// 3. Setup OAuth Config for Refreshing
		// We need this to rebuild the TokenSource
		config := &oauth2.Config{
			ClientID:     s.Spotify.ClientID,
			ClientSecret: s.Spotify.ClientSecret,
			Endpoint:     oauthSpotify.Endpoint,
		}

		// 4. Refresh Token Logic
		// We create a 'TokenSource'. This interface automatically handles
		// checking if the token is expired and refreshing it if needed.
		initialToken := conn.ToOAuthToken()
		tokenSource := config.TokenSource(r.Context(), initialToken)

		// Get the token (this forces the refresh if it was expired)
		newToken, err := tokenSource.Token()
		if err != nil {
			log.Printf("Failed to refresh token: %v", err)
			http.Error(w, "Token expired and refresh failed. Please login again.", 401)
			return
		}

		// 5. Check if it changed, and Save if necessary
		if newToken.AccessToken != initialToken.AccessToken ||
			(newToken.RefreshToken != "" && newToken.RefreshToken != initialToken.RefreshToken) {

			log.Println("Token was refreshed! Saving new token to DB...")

			// Handle the edge case where some providers don't send back a
			// new refresh token if the old one is still valid.
			if newToken.RefreshToken == "" {
				newToken.RefreshToken = initialToken.RefreshToken
			}

			err := s.Store.SaveSpotifyConnection(r.Context(), userID, conn.ProviderID, newToken)
			if err != nil {
				log.Printf("CRITICAL: Failed to save refreshed token: %v", err)
				// We continue anyway, because we have the valid token in memory now
			}
		}

		// 6. Start the Sync
		// We pass the valid 'newToken' to the sync engine
		log.Println("Starting Spotify Sync...")
		err = s.Spotify.SyncAll(r.Context(), newToken, s.Store, userID)
		if err != nil {
			log.Printf("Sync Error: %v", err)
			http.Error(w, "Sync failed check logs", 500)
			return
		}

		w.Write([]byte("Sync Complete! Check your database."))
	}
}
