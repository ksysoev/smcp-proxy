package auth

import (
	"context"
	"log/slog"
	"net/http"
)

// NoAuthTokenClient is a no-op implementation of TokenClient that doesn't perform any authentication
type NoAuthTokenClient struct{}

// NewNoAuthTokenClient creates a new NoAuthTokenClient
func NewNoAuthTokenClient() *NoAuthTokenClient {
	return &NoAuthTokenClient{}
}

// GetToken always returns an empty token
func (c *NoAuthTokenClient) GetToken(_ context.Context) (string, error) {
	return "", nil
}

// NoAuthTransport is an HTTP transport that doesn't add any authentication
type NoAuthTransport struct {
	Base   http.RoundTripper
	Logger *slog.Logger
}

// RoundTrip implements the http.RoundTripper interface
func (t *NoAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Don't add any authentication headers
	return t.Base.RoundTrip(req)
}