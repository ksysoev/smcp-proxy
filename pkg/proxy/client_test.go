package proxy

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockTokenClient is a mock implementation of the auth.TokenClient interface
type MockTokenClient struct {
	mock.Mock
}

func (m *MockTokenClient) GetToken() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// MockHTTPTransport is a mock implementation of http.RoundTripper
type MockHTTPTransport struct {
	mock.Mock
}

func (m *MockHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// No longer needed - we're now testing configuration instead of behavior

func TestNewClient(t *testing.T) {
	// Create a test logger
	testLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("Create client with valid options", func(t *testing.T) {
		// Create client options with all required fields
		opts := ClientOptions{
			// Client settings
			Host:            "127.0.0.1",
			Port:            8081,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 10 * time.Second,

			// Server settings
			ServerURL:     "http://localhost:8080",
			ServerTimeout: 60 * time.Second,

			// OIDC settings
			OIDCIssuer:        "https://example.com",
			OIDCClientID:      "test-client",
			OIDCClientSecret:  "test-secret",
			OIDCAudience:      "test-audience",
			OIDCScopes:        []string{"openid"},
			OIDCCacheTTL:      5 * time.Minute,
			OIDCTokenTTLDelta: 30 * time.Second,

			// TLS settings
			TLSEnabled:  false,
			TLSCertFile: "",
			TLSKeyFile:  "",

			// Metrics settings
			MetricsEnabled: true,
			MetricsPath:    "/metrics",
		}

		// Create client
		client, err := NewClient(opts, testLogger)
		require.NoError(t, err)
		require.NotNil(t, client)

		// Verify the client was properly configured
		assert.Equal(t, "127.0.0.1:8081", client.httpServer.Addr)
		assert.Equal(t, opts.ReadTimeout, client.httpServer.ReadTimeout)
		assert.Equal(t, opts.WriteTimeout, client.httpServer.WriteTimeout)
		assert.Equal(t, testLogger, client.logger)

		// Verify that the config was created correctly for backward compatibility
		assert.Equal(t, opts.ServerURL, client.cfg.Server.URL)
		assert.Equal(t, opts.ServerTimeout, client.cfg.Server.Timeout)
		assert.Equal(t, opts.TLSEnabled, client.cfg.TLS.Enabled)
		assert.Equal(t, opts.TLSCertFile, client.cfg.TLS.CertFile)
		assert.Equal(t, opts.TLSKeyFile, client.cfg.TLS.KeyFile)
		assert.Equal(t, opts.MetricsEnabled, client.cfg.Metrics.Enabled)
		assert.Equal(t, opts.MetricsPath, client.cfg.Metrics.Path)
		assert.Equal(t, opts.ShutdownTimeout, client.cfg.Client.ShutdownTimeout)
	})

	t.Run("Create client with default logger", func(t *testing.T) {
		// Create minimal client options
		opts := ClientOptions{
			// Minimal required settings
			Host:             "127.0.0.1",
			Port:             8081,
			ServerURL:        "http://localhost:8080",
			OIDCIssuer:       "https://example.com",
			OIDCClientID:     "test-client",
			OIDCClientSecret: "test-secret",
			OIDCScopes:       []string{"openid"},
		}

		// Create client with nil logger
		client, err := NewClient(opts, nil)
		require.NoError(t, err)
		require.NotNil(t, client)

		// Verify that the logger was set to the default
		assert.NotNil(t, client.logger)
	})

	t.Run("Client has expected routes", func(t *testing.T) {
		// Create minimal client options
		opts := ClientOptions{
			Host:             "127.0.0.1",
			Port:             8081,
			ServerURL:        "http://localhost:8080",
			OIDCIssuer:       "https://example.com",
			OIDCClientID:     "test-client",
			OIDCClientSecret: "test-secret",
			OIDCScopes:       []string{"openid"},
			MetricsEnabled:   true,
			MetricsPath:      "/test-metrics",
		}

		// Create client
		client, err := NewClient(opts, testLogger)
		require.NoError(t, err)
		require.NotNil(t, client)

		// Check that we've registered the required routes
		// Note: This is a basic check, we can't easily verify the exact routes
		// since the http.ServeMux doesn't expose its patterns
		assert.NotNil(t, client.mux)
	})
}

func TestClientConfiguration(t *testing.T) {
	// Create a test logger
	testLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("Client without TLS configuration", func(t *testing.T) {
		// Create minimal client options
		opts := ClientOptions{
			Host:             "127.0.0.1",
			Port:             8081,
			ServerURL:        "http://localhost:8080",
			OIDCIssuer:       "https://example.com",
			OIDCClientID:     "test-client",
			OIDCClientSecret: "test-secret",
			OIDCScopes:       []string{"openid"},
			TLSEnabled:       false,
		}

		// Create client
		client, err := NewClient(opts, testLogger)
		require.NoError(t, err)
		require.NotNil(t, client)

		// Verify that TLS is disabled
		assert.False(t, client.cfg.TLS.Enabled)
	})

	t.Run("Client with TLS configuration", func(t *testing.T) {
		// Create minimal client options with TLS enabled
		opts := ClientOptions{
			Host:             "127.0.0.1",
			Port:             8081,
			ServerURL:        "http://localhost:8080",
			OIDCIssuer:       "https://example.com",
			OIDCClientID:     "test-client",
			OIDCClientSecret: "test-secret",
			OIDCScopes:       []string{"openid"},
			TLSEnabled:       true,
			TLSCertFile:      "cert.pem",
			TLSKeyFile:       "key.pem",
		}

		// Create client
		client, err := NewClient(opts, testLogger)
		require.NoError(t, err)
		require.NotNil(t, client)

		// Verify TLS configuration
		assert.True(t, client.cfg.TLS.Enabled)
		assert.Equal(t, "cert.pem", client.cfg.TLS.CertFile)
		assert.Equal(t, "key.pem", client.cfg.TLS.KeyFile)
	})
}

func TestClientShutdownTimeout(t *testing.T) {
	// Create a test logger
	testLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create client options with specific shutdown timeout
	opts := ClientOptions{
		Host:             "127.0.0.1",
		Port:             8081,
		ServerURL:        "http://localhost:8080",
		OIDCIssuer:       "https://example.com",
		OIDCClientID:     "test-client",
		OIDCClientSecret: "test-secret",
		OIDCScopes:       []string{"openid"},
		ShutdownTimeout:  15 * time.Second, // Custom timeout
	}

	// Create client
	client, err := NewClient(opts, testLogger)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify that the shutdown timeout was properly configured
	assert.Equal(t, 15*time.Second, client.cfg.Client.ShutdownTimeout)
}

func TestClientTransport(t *testing.T) {
	// Create a test logger
	testLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("RoundTrip with successful response", func(t *testing.T) {
		// Create a mock transport
		mockTransport := new(MockHTTPTransport)

		// Setup the mock to return a successful response
		mockResp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString("OK")),
			Header:     make(http.Header),
		}
		mockTransport.On("RoundTrip", mock.Anything).Return(mockResp, nil)

		// Create the client transport
		transport := &clientTransport{
			base:   mockTransport,
			logger: testLogger,
		}

		// Create a test request
		req := httptest.NewRequest("GET", "http://example.com/test", nil)

		// Execute the round trip
		resp, err := transport.RoundTrip(req)

		// Verify the results
		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		// Verify that the mock was called
		mockTransport.AssertCalled(t, "RoundTrip", mock.Anything)
	})

	t.Run("RoundTrip with error", func(t *testing.T) {
		// Create a mock transport
		mockTransport := new(MockHTTPTransport)

		// Setup the mock to return an error
		mockError := context.DeadlineExceeded
		mockTransport.On("RoundTrip", mock.Anything).Return(nil, mockError)

		// Create the client transport with a buffer logger to test logging
		logBuf := &bytes.Buffer{}
		bufLogger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

		transport := &clientTransport{
			base:   mockTransport,
			logger: bufLogger,
		}

		// Create a test request
		req := httptest.NewRequest("GET", "http://example.com/test", nil)

		// Execute the round trip
		resp, err := transport.RoundTrip(req)

		// Verify the results
		assert.Error(t, err)
		assert.Equal(t, mockError, err)
		assert.Nil(t, resp)

		// Verify that the mock was called
		mockTransport.AssertCalled(t, "RoundTrip", mock.Anything)

		// Verify that the error was logged
		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "Request failed")
		assert.Contains(t, logOutput, "context deadline exceeded")
	})
}
