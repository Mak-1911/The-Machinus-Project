// Package oauth provides OAuth types for the UI.
package oauth

import "time"

// Provider represents an OAuth provider.
type Provider string

const (
	ProviderAnthropic Provider = "anthropic"
	ProviderGitHub   Provider = "github"
	ProviderCopilot  Provider = "copilot"
	ProviderHyper    Provider = "hyper"
)

// Token is an alias for OAuthToken.
type Token = OAuthToken

// OAuthToken represents an OAuth token.
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Provider     Provider  `json:"provider"`
}

// IsValid checks if the token is still valid.
func (t *OAuthToken) IsValid() bool {
	if t == nil || t.AccessToken == "" {
		return false
	}
	// Add buffer to expiry
	return time.Now().Add(5 * time.Minute).Before(t.ExpiresAt)
}

// OAuthConfig represents OAuth configuration.
type OAuthConfig struct {
	ClientID     string   `json:"client_id"`
	RedirectURL  string   `json:"redirect_url"`
	Scopes       []string `json:"scopes"`
	AuthURL      string   `json:"auth_url"`
	TokenURL     string   `json:"token_url"`
	Provider     Provider `json:"provider"`
}

// OAuthState represents the state during OAuth flow.
type OAuthState struct {
	State      string    `json:"state"`
	Provider   Provider  `json:"provider"`
	CreatedAt  time.Time `json:"created_at"`
	Verifier   string    `json:"verifier,omitempty"`
	RedirectCh chan bool `json:"-"` // Channel to signal completion
}
