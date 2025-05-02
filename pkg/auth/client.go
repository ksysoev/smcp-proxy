package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2/clientcredentials"
)

// TokenClient is responsible for acquiring OIDC tokens
type TokenClient interface {
	// GetToken returns a valid token, refreshing if necessary
	GetToken(ctx context.Context) (string, error)
}

// OIDCTokenClient implements TokenClient using the client credentials flow
type OIDCTokenClient struct {
	tokenExpiry   time.Time
	config        *clientcredentials.Config
	logger        *slog.Logger
	tokenCache    string
	cacheTTL      time.Duration
	tokenTTLDelta time.Duration
	mu            sync.RWMutex
}

// NewOIDCTokenClient creates a new OIDC token client
func NewOIDCTokenClient(
	issuerURL string,
	clientID string,
	clientSecret string,
	audience string,
	scopes []string,
	cacheTTL time.Duration,
	tokenTTLDelta time.Duration,
	logger *slog.Logger,
) *OIDCTokenClient {
	if logger == nil {
		logger = slog.Default()
	}

	// Create the OAuth2 config
	config := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     issuerURL + "/oauth/token",
		Scopes:       scopes,
		EndpointParams: map[string][]string{
			"audience": {audience},
		},
	}

	return &OIDCTokenClient{
		config:        config,
		cacheTTL:      cacheTTL,
		tokenTTLDelta: tokenTTLDelta,
		logger:        logger,
	}
}

// GetToken returns a valid token, refreshing if necessary
func (c *OIDCTokenClient) GetToken(ctx context.Context) (string, error) {
	// Check if we have a cached token that's still valid
	c.mu.RLock()
	tokenCache := c.tokenCache
	tokenExpiry := c.tokenExpiry
	c.mu.RUnlock()

	// If we have a valid token in cache, return it
	now := time.Now()
	if tokenCache != "" && tokenExpiry.After(now.Add(c.tokenTTLDelta)) {
		return tokenCache, nil
	}

	// Need to refresh the token
	token, err := c.refreshToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}

	return token, nil
}

// refreshToken acquires a new token using the client credentials flow
func (c *OIDCTokenClient) refreshToken(ctx context.Context) (string, error) {
	// Request a new token
	token, err := c.config.Token(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	// Cache the token
	c.mu.Lock()
	c.tokenCache = token.AccessToken
	c.tokenExpiry = token.Expiry
	c.mu.Unlock()

	c.logger.Debug("Refreshed OIDC token",
		"expiry", token.Expiry.Format(time.RFC3339),
		"ttl", time.Until(token.Expiry).Round(time.Second))

	return token.AccessToken, nil
}

// TokenTransport is an http.RoundTripper that adds an OAuth2 token to the request
type TokenTransport struct {
	Base        http.RoundTripper
	Client      TokenClient
	Logger      *slog.Logger
	LastToken   string
	CacheErrors bool
}

// RoundTrip implements http.RoundTripper, adding an OAuth2 token to the request
func (t *TokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Get a token
	token, err := t.Client.GetToken(req.Context())
	if err != nil {
		if t.CacheErrors && t.LastToken != "" {
			// Use the last token if we can't get a new one
			token = t.LastToken
			if t.Logger != nil {
				t.Logger.Warn("Using cached token due to error getting new token", "error", err)
			}
		} else {
			return nil, fmt.Errorf("failed to get token: %w", err)
		}
	} else {
		// Update the last token
		t.LastToken = token
	}

	// Clone the request to avoid modifying the original
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+token)

	// Use the base transport to make the request
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req2)
}
