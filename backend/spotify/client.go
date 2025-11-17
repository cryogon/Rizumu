package spotify

import (
	"context"
	"net/http"

	"github.com/zmb3/spotify/v2"
	auth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

const redirectURL = "http://localhost:8080/auth/spotify/callback"

// These are the permissions we'll ask the user for.
var scopes = []string{
	auth.ScopePlaylistReadPrivate,
	auth.ScopePlaylistReadCollaborative,
	auth.ScopeUserReadEmail,
	auth.ScopeUserReadPrivate,
}

// Client is our wrapper for the Spotify authenticator and client.
type Client struct {
	auth *auth.Authenticator
}

// NewClient creates our client
func NewClient(clientID, clientSecret string) *Client {
	// 1. Create the authenticator
	auth := auth.New(
		auth.WithRedirectURL(redirectURL),
		auth.WithScopes(scopes...),
		auth.WithClientID(clientID),
		auth.WithClientSecret(clientSecret),
	)

	return &Client{
		auth: auth,
	}
}

// GetAuthURL creates a unique login URL for the user.
func (c *Client) GetAuthURL(state string) string {
	// The 'state' is a random string you generate to prevent CSRF attacks.
	return c.auth.AuthURL(state)
}

// ExchangeCode is what you call in your /callback handler
func (c *Client) ExchangeCode(r *http.Request, state string) (*oauth2.Token, error) {
	// This gets the 'code' from the URL
	token, err := c.auth.Token(r.Context(), state, r)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// NewClientFromToken creates a *real* Spotify client
// that can make API calls (like getting playlists).
func (c *Client) NewClientFromToken(token *oauth2.Token) *spotify.Client {
	// This creates an HTTP client that automatically adds the
	// "Authorization: Bearer <token>" header to every request.
	httpClient := c.auth.Client(context.Background(), token)

	// Wrap that http client with the spotify.Client
	client := spotify.New(httpClient)
	return client
}
