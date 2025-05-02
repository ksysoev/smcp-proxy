package config

import (
	"os"
	"testing"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServerConfig(t *testing.T) {
	t.Run("Valid config file", func(t *testing.T) {
		// Create a temporary config file
		configContent := `
server:
  host: "127.0.0.1"
  port: 8888
  read_timeout: "15s"
  write_timeout: "20s"
  shutdown_timeout: "5s"

mcp:
  timeout: "45s"
  backends:
    - id: "test-backend"
      name: "Test Backend"
      model: "claude-3-haiku"
      max_tokens: 100000
      transport: "http"
      url: "http://localhost:9000"
      path: "/api"
      strip_path: true
      timeout: "30s"

oidc:
  issuers:
    - "https://example.com"
  audience: "test-audience"
  required_claims:
    role: "admin"
  optional_claims:
    scope: "read:data"

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
		config, err := NewServerConfig(configPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Verify server settings
		assert.Equal(t, "127.0.0.1", config.Server.Host)
		assert.Equal(t, 8888, config.Server.Port)
		assert.Equal(t, 15*time.Second, config.Server.ReadTimeout)
		assert.Equal(t, 20*time.Second, config.Server.WriteTimeout)
		assert.Equal(t, 5*time.Second, config.Server.ShutdownTimeout)

		// Verify MCP settings
		assert.Equal(t, 45*time.Second, config.MCP.Timeout)
		require.Len(t, config.MCP.Backends, 1)

		backend := config.MCP.Backends[0]
		assert.Equal(t, "test-backend", backend.ID)
		assert.Equal(t, "Test Backend", backend.Name)
		assert.Equal(t, "claude-3-haiku", backend.Model)
		assert.Equal(t, 100000, backend.MaxTokens)
		assert.Equal(t, HTTPTransport, backend.Transport)
		assert.Equal(t, "http://localhost:9000", backend.URL)
		assert.Equal(t, "/api", backend.Path)
		assert.True(t, backend.StripPath)
		assert.Equal(t, 30*time.Second, backend.Timeout)

		// Verify OIDC settings
		require.Len(t, config.OIDC.Issuers, 1)
		assert.Equal(t, "https://example.com", config.OIDC.Issuers[0])
		assert.Equal(t, "test-audience", config.OIDC.Audience)
		assert.Equal(t, "admin", config.OIDC.RequiredClaims["role"])
		assert.Equal(t, "read:data", config.OIDC.OptionalClaims["scope"])

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
server:
  host: "127.0.0.1"
  port: 8080
  
mcp:
  timeout: "60s"
  backends:
    - id: "test-backend"
      name: "Test Backend"
      transport: "http"
      url: "http://localhost:9000"
      path: "/api"
`
		configPath := test.TempFile(t, configContent)

		// Set environment variables to override config
		test.SetEnv(t, "SMCP_PROXY_SERVER_PORT", "9999")
		test.SetEnv(t, "SMCP_PROXY_MCP_TIMEOUT", "120s")

		// Load the config
		config, err := NewServerConfig(configPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Verify environment variables took precedence
		assert.Equal(t, 9999, config.Server.Port)
		assert.Equal(t, 120*time.Second, config.MCP.Timeout)
	})

	t.Run("Default values are set", func(t *testing.T) {
		// Create a minimal config file
		configContent := `
mcp:
  backends:
    - id: "test-backend"
      name: "Test Backend"
      transport: "http"
      url: "http://localhost:9000"
      path: "/api"
`
		configPath := test.TempFile(t, configContent)

		// Load the config
		config, err := NewServerConfig(configPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Verify default values
		assert.Equal(t, "0.0.0.0", config.Server.Host)
		assert.Equal(t, 8080, config.Server.Port)
		assert.Equal(t, 30*time.Second, config.Server.ReadTimeout)
		assert.Equal(t, 30*time.Second, config.Server.WriteTimeout)
		assert.Equal(t, 10*time.Second, config.Server.ShutdownTimeout)
		assert.Equal(t, 60*time.Second, config.MCP.Timeout)
		assert.False(t, config.TLS.Enabled)
		assert.True(t, config.Metrics.Enabled)
		assert.Equal(t, "/metrics", config.Metrics.Path)
	})

	t.Run("Invalid config file", func(t *testing.T) {
		// Create a temporary file with invalid YAML
		configPath := test.TempFile(t, "this is not valid YAML")

		// Attempt to load the config
		config, err := NewServerConfig(configPath)
		require.Error(t, err)
		require.Nil(t, config)
	})

	t.Run("Non-existent config file", func(t *testing.T) {
		// Use a path that doesn't exist
		configPath := "/nonexistent/config.yml"

		// Attempt to load the config
		config, err := NewServerConfig(configPath)
		require.Error(t, err)
		require.Nil(t, config)
	})
}
