package config

import (
	"testing"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientConfig(t *testing.T) {
	t.Run("Valid config file", func(t *testing.T) {
		// Create a temporary config file
		configContent := `
client:
  host: "127.0.0.1"
  port: 8081
  read_timeout: "15s"
  write_timeout: "20s"
  shutdown_timeout: "5s"

server:
  url: "https://proxy-server.example.com"
  timeout: "45s"

oidc:
  issuer: "https://example.com"
  client_id: "test-client"
  client_secret: "test-secret"
  audience: "test-audience"
  scopes:
    - "openid"
    - "profile"
  cache_ttl: "10m"
  token_ttl_delta: "60s"

tls:
  enabled: true
  cert_file: "/path/to/cert.pem"
  key_file: "/path/to/key.pem"

metrics:
  enabled: false
  path: "/custom-metrics"
`
		configPath := test.TempFile(t, configContent)

		// Load the config
		config, err := NewClientConfig(configPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Verify client settings
		assert.Equal(t, "127.0.0.1", config.Client.Host)
		assert.Equal(t, 8081, config.Client.Port)
		assert.Equal(t, 15*time.Second, config.Client.ReadTimeout)
		assert.Equal(t, 20*time.Second, config.Client.WriteTimeout)
		assert.Equal(t, 5*time.Second, config.Client.ShutdownTimeout)

		// Verify server settings
		assert.Equal(t, "https://proxy-server.example.com", config.Server.URL)
		assert.Equal(t, 45*time.Second, config.Server.Timeout)

		// Verify OIDC settings
		assert.Equal(t, "https://example.com", config.OIDC.Issuer)
		assert.Equal(t, "test-client", config.OIDC.ClientID)
		assert.Equal(t, "test-secret", config.OIDC.ClientSecret)
		assert.Equal(t, "test-audience", config.OIDC.Audience)
		require.Len(t, config.OIDC.Scopes, 2)
		assert.Equal(t, "openid", config.OIDC.Scopes[0])
		assert.Equal(t, "profile", config.OIDC.Scopes[1])
		assert.Equal(t, 10*time.Minute, config.OIDC.CacheTTL)
		assert.Equal(t, 60*time.Second, config.OIDC.TokenTTLDelta)

		// Verify TLS settings
		assert.True(t, config.TLS.Enabled)
		assert.Equal(t, "/path/to/cert.pem", config.TLS.CertFile)
		assert.Equal(t, "/path/to/key.pem", config.TLS.KeyFile)

		// Verify metrics settings
		assert.False(t, config.Metrics.Enabled)
		assert.Equal(t, "/custom-metrics", config.Metrics.Path)
	})

	t.Run("Environment variables override config", func(t *testing.T) {
		// Create a basic config file
		configContent := `
client:
  host: "127.0.0.1"
  port: 8081
  
server:
  url: "http://localhost:8080"
  timeout: "60s"

oidc:
  issuer: "https://example.com"
  client_id: "test-client"
  client_secret: "test-secret"
  audience: "test-audience"
`
		configPath := test.TempFile(t, configContent)

		// Set environment variables to override config
		test.SetEnv(t, "SMCP_CLIENT_CLIENT_PORT", "9999")
		test.SetEnv(t, "SMCP_CLIENT_SERVER_URL", "https://override.example.com")

		// Load the config
		config, err := NewClientConfig(configPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Verify environment variables took precedence
		assert.Equal(t, 9999, config.Client.Port)
		assert.Equal(t, "https://override.example.com", config.Server.URL)
	})

	t.Run("Default values are set", func(t *testing.T) {
		// Create a minimal config file
		configContent := `
server:
  url: "http://localhost:8080"

oidc:
  issuer: "https://example.com"
  client_id: "test-client"
  client_secret: "test-secret"
  audience: "test-audience"
`
		configPath := test.TempFile(t, configContent)

		// Load the config
		config, err := NewClientConfig(configPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Verify default values
		assert.Equal(t, "127.0.0.1", config.Client.Host)
		assert.Equal(t, 8081, config.Client.Port)
		assert.Equal(t, 30*time.Second, config.Client.ReadTimeout)
		assert.Equal(t, 30*time.Second, config.Client.WriteTimeout)
		assert.Equal(t, 10*time.Second, config.Client.ShutdownTimeout)
		assert.Equal(t, 60*time.Second, config.Server.Timeout)
		assert.Equal(t, []string{"openid"}, config.OIDC.Scopes)
		assert.Equal(t, 5*time.Minute, config.OIDC.CacheTTL)
		assert.Equal(t, 30*time.Second, config.OIDC.TokenTTLDelta)
		assert.False(t, config.TLS.Enabled)
		assert.True(t, config.Metrics.Enabled)
		assert.Equal(t, "/metrics", config.Metrics.Path)
	})

	t.Run("Required fields validation", func(t *testing.T) {
		// Test missing required fields one by one
		testCases := []struct {
			name          string
			configContent string
		}{
			{
				name: "Missing issuer",
				configContent: `
server:
  url: "http://localhost:8080"

oidc:
  # issuer missing
  client_id: "test-client"
  client_secret: "test-secret"
  audience: "test-audience"
`,
			},
			{
				name: "Missing client ID",
				configContent: `
server:
  url: "http://localhost:8080"

oidc:
  issuer: "https://example.com"
  # client_id missing
  client_secret: "test-secret"
  audience: "test-audience"
`,
			},
			{
				name: "Missing client secret",
				configContent: `
server:
  url: "http://localhost:8080"

oidc:
  issuer: "https://example.com"
  client_id: "test-client"
  # client_secret missing
  audience: "test-audience"
`,
			},
			{
				name: "Missing server URL",
				configContent: `
server:
  # url missing
  
oidc:
  issuer: "https://example.com"
  client_id: "test-client"
  client_secret: "test-secret"
  audience: "test-audience"
`,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				configPath := test.TempFile(t, tc.configContent)

				// Attempt to load the config
				config, err := NewClientConfig(configPath)
				require.Error(t, err, "Expected validation error for "+tc.name)
				require.Nil(t, config)
			})
		}
	})

	t.Run("Invalid config file", func(t *testing.T) {
		// Create a temporary file with invalid YAML
		configPath := test.TempFile(t, "this is not valid YAML")

		// Attempt to load the config
		config, err := NewClientConfig(configPath)
		require.Error(t, err)
		require.Nil(t, config)
	})

	t.Run("Non-existent config file", func(t *testing.T) {
		// Use a path that doesn't exist
		configPath := "/nonexistent/config.yml"

		// Attempt to load the config
		config, err := NewClientConfig(configPath)
		require.Error(t, err)
		require.Nil(t, config)
	})
}
