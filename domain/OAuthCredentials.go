package domain

import "time"

// OAuthCredentials holds info for acquiring or refreshing an OAuth access token.
type OAuthCredentials struct {
	ClientID     string
	ClientSecret string
	TokenURL     string   // e.g. "https://auth.example.com/oauth2/token"
	Scopes       []string // e.g. ["read", "write"]

	AccessToken string    // The current valid access token
	TokenType   string    // Typically "Bearer", depends on auth server
	TokenExpiry time.Time // When this token expires (UTC)
}
