package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthModeConstants(t *testing.T) {
	t.Run("Auth mode constants have correct values", func(t *testing.T) {
		assert.Equal(t, AuthMode("oidc"), OIDCAuthMode)
		assert.Equal(t, AuthMode("none"), NoAuthMode)
	})
	
	t.Run("Default auth mode should be none in server config", func(t *testing.T) {
		// Create a minimal config file for testing defaults
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
		assert.NoError(t, err)
		assert.NotNil(t, config)

		// Verify default auth mode is "none"
		assert.Equal(t, NoAuthMode, config.Auth.Mode)
	})

	t.Run("Auth mode can be set to oidc in config", func(t *testing.T) {
		// Create a config file with auth mode set to oidc
		configContent := `
auth:
  mode: "oidc"

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
		assert.NoError(t, err)
		assert.NotNil(t, config)

		// Verify auth mode is set to "oidc"
		assert.Equal(t, OIDCAuthMode, config.Auth.Mode)
	})

	t.Run("Auth mode can be set via environment variable", func(t *testing.T) {
		// Create a config file with default auth mode
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

		// Set environment variable to override auth mode
		test.SetEnv(t, "SMCP_PROXY_AUTH_MODE", "oidc")

		// Load the config
		config, err := NewServerConfig(configPath)
		assert.NoError(t, err)
		assert.NotNil(t, config)

		// Verify auth mode is set to "oidc" from environment variable
		assert.Equal(t, OIDCAuthMode, config.Auth.Mode)
	})
}