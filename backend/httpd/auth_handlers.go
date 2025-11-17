package httpd

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"time"
)

const spotifyStateCookie = "rizumu-spotify-state"

func (s *Server) handleSpotifyLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, err := generateRandomState()
		if err != nil {
			log.Printf("ERROR: could not generate state: %v", err)
			http.Error(w, "Failed to generate security state", 500)
			return
		}

		cookie := &http.Cookie{
			Name:     spotifyStateCookie,
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			MaxAge:   int(time.Minute.Seconds() * 10),
		}

		http.SetCookie(w, cookie)
		authURL := s.Spotify.GetAuthURL(state)
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

func (s *Server) handleSpotifyCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryState := r.URL.Query().Get("state")
		if queryState == "" {
			http.Error(w, "Spotify didn't return a state", 400)
			return
		}

		cookie, err := r.Cookie(spotifyStateCookie)
		if err != nil {
			http.Error(w, "Security Cookie not found", 400)
			return
		}

		if queryState != cookie.Value {
			log.Printf("WARN: Invalid State. Cookie: %s, Query: %s", cookie.Value, queryState)
			http.Error(w, "Invalid security cookie", http.StatusForbidden)
			return
		}

		token, err := s.Spotify.ExchangeCode(r, cookie.Value)
		if err != nil {
			log.Printf("ERROR: Failed to exchange code: %v", err)
			http.Error(w, "Failed to get token from spotify", 500)
			return
		}

		client := s.Spotify.NewClientFromToken(token)
		spotifyUser, err := client.CurrentUser(r.Context())
		if err != nil {
			http.Error(w, "Failed to get spotify profile", 500)
			return
		}

		userID, err := s.Store.CreateAdminUser(r.Context())
		if err != nil {
			http.Error(w, "DB Error", 500)
			return
		}

		err = s.Store.SaveSpotifyConnection(r.Context(), userID, spotifyUser.ID, token)
		if err != nil {
			http.Error(w, "Failed to save to DB", 500)
			return
		}

		log.Printf("Successfully authenticated! Token: %s", token.AccessToken)
		w.Write([]byte("Success! You are authenticated. You can close this window."))
	}
}

func generateRandomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
