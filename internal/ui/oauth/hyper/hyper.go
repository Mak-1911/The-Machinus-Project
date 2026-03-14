// Package hyper provides Hyper OAuth types.
package hyper

import (
	"context"
	"time"

	"github.com/machinus/cloud-agent/internal/ui/oauth"
)

// AuthResponse represents the response from initiating device auth.
type AuthResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenIntrospect represents token introspection result.
type TokenIntrospect struct {
	Active bool `json:"active"`
}

// InitiateDeviceAuth initiates device auth.
func InitiateDeviceAuth(ctx context.Context) (*AuthResponse, error) {
	return nil, nil
}

// PollForToken polls for a token, returns authorization code when ready.
func PollForToken(ctx context.Context, deviceCode string, interval int) (string, error) {
	return "", nil
}

// ExchangeToken exchanges a token.
func ExchangeToken(ctx context.Context, code string) (*oauth.Token, error) {
	return &oauth.Token{
		AccessToken: "",
		ExpiresAt:   time.Now(),
		Provider:    oauth.ProviderHyper,
	}, nil
}

// IntrospectToken introspects a token.
func IntrospectToken(ctx context.Context, token string) (*TokenIntrospect, error) {
	return &TokenIntrospect{Active: true}, nil
}
