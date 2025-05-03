package cmd

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/ksysoev/smcp-proxy/pkg/test"
	"github.com/stretchr/testify/assert"
)

func TestClientEnvVars(t *testing.T) {
	// Save original values of variables we'll modify
	origServerURL := serverURL
	origOidcIssuer := oidcIssuer
	origOidcClientID := oidcClientID
	origOidcClientSecret := oidcClientSecret
	origOidcScopes := oidcScopes
	origClientHost := clientHost
	origClientPort := clientPort
	origAuthMode := authMode

	// Restore original values when the test finishes
	defer func() {
		serverURL = origServerURL
		oidcIssuer = origOidcIssuer
		oidcClientID = origOidcClientID
		oidcClientSecret = origOidcClientSecret
		oidcScopes = origOidcScopes
		clientHost = origClientHost
		clientPort = origClientPort
		authMode = origAuthMode
	}()

	t.Run("Environment variables are applied", func(t *testing.T) {
		// Reset variables to their defaults or empty values
		serverURL = ""
		oidcIssuer = ""
		oidcClientID = ""
		oidcClientSecret = ""
		oidcScopes = "openid"
		clientHost = "127.0.0.1"
		clientPort = 8081
		authMode = "" // Empty auth mode so it will pick up env var

		// Set environment variables
		test.SetEnv(t, "SMCP_SERVER_URL", "https://env-server.example.com")
		test.SetEnv(t, "SMCP_AUTH_MODE", "oidc")
		test.SetEnv(t, "SMCP_OIDC_ISSUER", "https://env-issuer.example.com")
		test.SetEnv(t, "SMCP_OIDC_CLIENT_ID", "env-client-id")
		test.SetEnv(t, "SMCP_OIDC_CLIENT_SECRET", "env-client-secret")
		test.SetEnv(t, "SMCP_OIDC_SCOPES", "openid,profile,email")
		test.SetEnv(t, "SMCP_CLIENT_HOST", "0.0.0.0")
		test.SetEnv(t, "SMCP_CLIENT_PORT", "9999")

		// Create a logger with a buffer so we can check output
		logBuf := &bytes.Buffer{}
		logger := setupTestLogger(logBuf)

		// Call checkEnvVars
		checkEnvVars(logger)

		// Verify environment variables were applied
		assert.Equal(t, "https://env-server.example.com", serverURL)
		assert.Equal(t, "oidc", authMode)
		assert.Equal(t, "https://env-issuer.example.com", oidcIssuer)
		assert.Equal(t, "env-client-id", oidcClientID)
		assert.Equal(t, "env-client-secret", oidcClientSecret)
		assert.Equal(t, "openid,profile,email", oidcScopes)
		assert.Equal(t, "0.0.0.0", clientHost)
		assert.Equal(t, 9999, clientPort)

		// Check that the logger recorded debug messages
		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "Using environment variable for server URL")
		assert.Contains(t, logOutput, "Using environment variable for auth mode")
		assert.Contains(t, logOutput, "Using environment variable for OIDC issuer")
	})

	t.Run("Auth mode from env var overrides command line value", func(t *testing.T) {
		// Set variables as if they were provided via command line
		serverURL = "https://cmd-server.example.com"
		authMode = "oidc"  // Command line auth mode
		oidcIssuer = "https://cmd-issuer.example.com"
		oidcClientID = "cmd-client-id"
		oidcClientSecret = "cmd-client-secret"
		oidcScopes = "cmd-scope1,cmd-scope2"
		clientHost = "localhost"
		clientPort = 8080

		// Set environment variables that should override auth mode only
		test.SetEnv(t, "SMCP_SERVER_URL", "https://env-server.example.com")
		test.SetEnv(t, "SMCP_AUTH_MODE", "none")  // Different auth mode in env
		test.SetEnv(t, "SMCP_OIDC_ISSUER", "https://env-issuer.example.com")
		test.SetEnv(t, "SMCP_OIDC_CLIENT_ID", "env-client-id")
		test.SetEnv(t, "SMCP_OIDC_CLIENT_SECRET", "env-client-secret")
		test.SetEnv(t, "SMCP_OIDC_SCOPES", "openid,profile,email")
		test.SetEnv(t, "SMCP_CLIENT_HOST", "0.0.0.0")
		test.SetEnv(t, "SMCP_CLIENT_PORT", "9999")

		// Create a logger with a buffer
		logBuf := &bytes.Buffer{}
		logger := setupTestLogger(logBuf)

		// Call checkEnvVars
		checkEnvVars(logger)

		// Verify auth mode is overridden but other values aren't
		assert.Equal(t, "https://cmd-server.example.com", serverURL) // Not overridden
		assert.Equal(t, "none", authMode)  // Overridden by env var
		assert.Equal(t, "https://cmd-issuer.example.com", oidcIssuer) // Not overridden
		assert.Equal(t, "cmd-client-id", oidcClientID) // Not overridden
		assert.Equal(t, "cmd-client-secret", oidcClientSecret) // Not overridden
		assert.Equal(t, "cmd-scope1,cmd-scope2", oidcScopes) // Not overridden
		assert.Equal(t, "localhost", clientHost) // Not overridden
		assert.Equal(t, 8080, clientPort) // Not overridden

		// Check that the logger recorded debug message for auth mode
		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "Using environment variable for auth mode")
	})

	t.Run("Required flags validation", func(t *testing.T) {
		// Test different combinations of missing required flags
		testCases := []struct {
			name      string
			serverURL string
			authMode  string
			issuer    string
			clientID  string
			secret    string
			expectErr bool
		}{
			{
				name:      "All required flags for OIDC auth mode",
				serverURL: "https://server.example.com",
				authMode:  "oidc",
				issuer:    "https://issuer.example.com",
				clientID:  "client-id",
				secret:    "client-secret",
				expectErr: false,
			},
			{
				name:      "Only server URL required for no-auth mode",
				serverURL: "https://server.example.com",
				authMode:  "none",
				issuer:    "",
				clientID:  "",
				secret:    "",
				expectErr: false,
			},
			{
				name:      "Missing server URL",
				serverURL: "",
				authMode:  "none",
				issuer:    "",
				clientID:  "",
				secret:    "",
				expectErr: true,
			},
			{
				name:      "Missing issuer in OIDC mode",
				serverURL: "https://server.example.com",
				authMode:  "oidc",
				issuer:    "",
				clientID:  "client-id",
				secret:    "client-secret",
				expectErr: true,
			},
			{
				name:      "Missing client ID in OIDC mode",
				serverURL: "https://server.example.com",
				authMode:  "oidc",
				issuer:    "https://issuer.example.com",
				clientID:  "",
				secret:    "client-secret",
				expectErr: true,
			},
			{
				name:      "Missing client secret in OIDC mode",
				serverURL: "https://server.example.com",
				authMode:  "oidc",
				issuer:    "https://issuer.example.com",
				clientID:  "client-id",
				secret:    "",
				expectErr: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Set the test values
				serverURL = tc.serverURL
				authMode = tc.authMode
				oidcIssuer = tc.issuer
				oidcClientID = tc.clientID
				oidcClientSecret = tc.secret

				// Run validation
				err := validateRequiredFlags()

				if tc.expectErr {
					assert.Error(t, err)
					if tc.serverURL == "" {
						assert.Contains(t, err.Error(), "server URL is required")
					} else if tc.authMode == "oidc" && tc.issuer == "" {
						assert.Contains(t, err.Error(), "OIDC issuer is required when auth-mode is 'oidc'")
					} else if tc.authMode == "oidc" && tc.clientID == "" {
						assert.Contains(t, err.Error(), "OIDC client ID is required when auth-mode is 'oidc'")
					} else if tc.authMode == "oidc" && tc.secret == "" {
						assert.Contains(t, err.Error(), "OIDC client secret is required when auth-mode is 'oidc'")
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Parse environment integer variables", func(t *testing.T) {
		result, err := parseEnvInt("123")
		assert.NoError(t, err)
		assert.Equal(t, 123, result)

		result, err = parseEnvInt("not a number")
		assert.Error(t, err)
		assert.Equal(t, 0, result)
	})
}

// Helper function to create a logger that writes to a buffer
func setupTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}