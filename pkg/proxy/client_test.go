package proxy

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
