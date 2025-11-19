package spotify

import (
	"context"
	"net/http"

	"github.com/zmb3/spotify/v2"
	auth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

const redirectURL = "http://localhost:8080/auth/spotify/callback"

var scopes = []string{
	auth.ScopePlaylistReadPrivate,
	auth.ScopePlaylistReadCollaborative,
	auth.ScopeUserReadEmail,
	auth.ScopeUserReadPrivate,
}

type Client struct {
	auth         *auth.Authenticator
	ClientID     string
	ClientSecret string
}

// NewClient creates our client
func NewClient(clientID, clientSecret string) *Client {
	auth := auth.New(
		auth.WithRedirectURL(redirectURL),
		auth.WithScopes(scopes...),
		auth.WithClientID(clientID),
		auth.WithClientSecret(clientSecret),
	)

	return &Client{
		auth:         auth,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
}

// GetAuthURL creates a unique login URL for the user.
func (c *Client) GetAuthURL(state string) string {
	// The 'state' is a random string you generate to prevent CSRF attacks.
	return c.auth.AuthURL(state)
}

func (c *Client) ExchangeCode(r *http.Request, state string) (*oauth2.Token, error) {
	token, err := c.auth.Token(r.Context(), state, r)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// NewClientFromToken creates a *real* Spotify client
// that can make API calls (like getting playlists).
func (c *Client) NewClientFromToken(token *oauth2.Token) *spotify.Client {
	httpClient := c.auth.Client(context.Background(), token)

	client := spotify.New(httpClient)
	return client
}

// GetUserPlaylists fetches the authenticated user's playlists
func (c *Client) GetUserPlaylists(ctx context.Context, token *oauth2.Token) ([]spotify.SimplePlaylist, error) {
	// Create a client using the saved token
	client := c.NewClientFromToken(token)

	// Fetch playlists (fetching first 50 for now)
	// In a real app, you'd handle pagination to get ALL of them.
	page, err := client.CurrentUsersPlaylists(ctx, spotify.Limit(50))
	if err != nil {
		return nil, err
	}

	return page.Playlists, nil
}
