// Package copilot provides GitHub Copilot OAuth types.
package copilot

import (
	"context"
	"time"

	"github.com/machinus/cloud-agent/internal/ui/oauth"
)

// DeviceCode represents a GitHub device code for OAuth flow.
type DeviceCode struct {
	DeviceCode      string    `json:"device_code"`
	UserCode        string    `json:"user_code"`
	VerificationURI string    `json:"verification_uri"`
	ExpiresIn       int       `json:"expires_in"`
	Interval        int       `json:"interval"`
	ExpiresAt       time.Time `json:"expires_at"`
}

// IsExpired checks if the device code has expired.
func (d *DeviceCode) IsExpired() bool {
	return time.Now().After(d.ExpiresAt)
}

// RequestDeviceCode requests a device code for OAuth flow.
func RequestDeviceCode(ctx context.Context) (*DeviceCode, error) {
	return nil, nil
}

// PollForToken polls for a token.
func PollForToken(ctx context.Context, dc *DeviceCode) (*oauth.Token, error) {
	return nil, nil
}
