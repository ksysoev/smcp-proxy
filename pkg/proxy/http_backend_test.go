package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/auth"
	"github.com/ksysoev/smcp-proxy/pkg/config"
	"github.com/ksysoev/smcp-proxy/pkg/test"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPBackendHandler(t *testing.T) {
	logger := test.NewTestLogger()

	t.Run("Create handler with valid config", func(t *testing.T) {
		backend := &config.MCPBackend{
			ID:        "test-http",
			Name:      "Test HTTP",
			Transport: config.HTTPTransport,
			URL:       "http://example.com",
			Path:      "/test",
			Model:     "claude-3",
			Timeout:   30 * time.Second,
		}

		handler, err := NewHTTPBackendHandler(backend, logger)
		require.NoError(t, err)
		require.NotNil(t, handler)

		assert.Equal(t, backend, handler.backend)
		assert.NotNil(t, handler.proxy)
	})

	t.Run("Missing URL", func(t *testing.T) {
		backend := &config.MCPBackend{
			ID:        "test-http-missing-url",
			Name:      "Test HTTP Missing URL",
			Transport: config.HTTPTransport,
			Path:      "/test",
		}

		handler, err := NewHTTPBackendHandler(backend, logger)
		require.Error(t, err)
		require.Nil(t, handler)

		// Verify error type
		assert.IsType(t, ErrInvalidBackendConfig(""), err)
	})

	t.Run("Invalid URL", func(t *testing.T) {
		backend := &config.MCPBackend{
			ID:        "test-http-invalid-url",
			Name:      "Test HTTP Invalid URL",
			Transport: config.HTTPTransport,
			URL:       "://invalid-url",
			Path:      "/test",
		}

		handler, err := NewHTTPBackendHandler(backend, logger)
		require.Error(t, err)
		require.Nil(t, handler)
	})

	t.Run("Handle request with path stripping", func(t *testing.T) {
		// Create a test server to check if we're receiving the correct request
		var receivedRequest *http.Request
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedRequest = r
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		// Create backend with path stripping
		backend := &config.MCPBackend{
			ID:        "test-http-path-strip",
			Name:      "Test HTTP Path Strip",
			Transport: config.HTTPTransport,
			URL:       testServer.URL,
			Path:      "/prefix",
			StripPath: true,
			Model:     "claude-3",
		}

		handler, err := NewHTTPBackendHandler(backend, logger)
		require.NoError(t, err)
		require.NotNil(t, handler)

		// Create a request with the prefix path
		req := test.NewTestRequest("GET", "/prefix/resource", nil)

		// Add claims to the context
		claims := jwt.MapClaims{
			"sub":   "user-123",
			"email": "user@example.com",
		}
		ctx := context.WithValue(req.Context(), auth.ClaimsContextKey, claims)
		req = req.WithContext(ctx)

		// Handle the request
		w := test.NewTestResponse()
		handler.Handle(w, req)

		// Verify the response code
		assert.Equal(t, http.StatusOK, w.Code)

		// Verify the request was made with the path stripped
		require.NotNil(t, receivedRequest)
		assert.Equal(t, "/resource", receivedRequest.URL.Path)

		// Verify the custom headers were set
		assert.Equal(t, "user-123", receivedRequest.Header.Get("X-Subject"))
		assert.Equal(t, "user@example.com", receivedRequest.Header.Get("X-Email"))
		assert.Equal(t, "test-http-path-strip", receivedRequest.Header.Get("X-Proxy-Backend"))
		assert.Equal(t, "claude-3", receivedRequest.Header.Get("X-Proxy-Model"))

		// Verify the Authorization header was removed
		assert.Empty(t, receivedRequest.Header.Get("Authorization"))
	})

	t.Run("Handle request without path stripping", func(t *testing.T) {
		// Create a test server to check if we're receiving the correct request
		var receivedRequest *http.Request
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedRequest = r
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		// Create backend without path stripping
		backend := &config.MCPBackend{
			ID:        "test-http-no-strip",
			Name:      "Test HTTP No Strip",
			Transport: config.HTTPTransport,
			URL:       testServer.URL,
			Path:      "/prefix",
			StripPath: false,
		}

		handler, err := NewHTTPBackendHandler(backend, logger)
		require.NoError(t, err)
		require.NotNil(t, handler)

		// Create a request with the prefix path
		req := test.NewTestRequest("GET", "/prefix/resource", nil)

		// Handle the request
		w := test.NewTestResponse()
		handler.Handle(w, req)

		// Verify the response code
		assert.Equal(t, http.StatusOK, w.Code)

		// Verify the request was made without stripping the path
		require.NotNil(t, receivedRequest)
		assert.Equal(t, "/prefix/resource", receivedRequest.URL.Path)
	})
}
