package config

import (
	"testing"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientAuthConfig(t *testing.T) {
	t.Run("Auth mode defaults to none in client config", func(t *testing.T) {
		// Create a minimal valid client config
		configContent := `
server:
  url: "http://localhost:8080"
  timeout: "60s"

oidc:
  issuer: "https://example.com"
  client_id: "test-client"
  client_secret: "test-secret"
`
		configPath := test.TempFile(t, configContent)

		// Load the config
		config, err := NewClientConfig(configPath)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Verify default auth mode is "none"
		assert.Equal(t, NoAuthMode, config.Auth.Mode)
	})

	t.Run("Config validation only checks OIDC when auth mode is oidc", func(t *testing.T) {
		// Case 1: No OIDC config with auth mode "none" should succeed
		configContent := `
auth:
  mode: "none"

server:
  url: "http://localhost:8080"
  timeout: "60s"
`
		configPath := test.TempFile(t, configContent)
		config, err := NewClientConfig(configPath)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, NoAuthMode, config.Auth.Mode)

		// Case 2: No OIDC config with auth mode "oidc" should fail
		configContent = `
auth:
  mode: "oidc"

server:
  url: "http://localhost:8080"
  timeout: "60s"
`
		configPath = test.TempFile(t, configContent)
		config, err = NewClientConfig(configPath)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "OIDC issuer is required when auth.mode is 'oidc'")

		// Case 3: OIDC config with auth mode "oidc" should succeed
		configContent = `
auth:
  mode: "oidc"

server:
  url: "http://localhost:8080"
  timeout: "60s"

oidc:
  issuer: "https://example.com"
  client_id: "test-client"
  client_secret: "test-secret"
`
		configPath = test.TempFile(t, configContent)
		config, err = NewClientConfig(configPath)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, OIDCAuthMode, config.Auth.Mode)
	})

	t.Run("Auth mode can be set via environment variable", func(t *testing.T) {
		// Create a config file with default auth mode and required OIDC fields
		configContent := `
server:
  url: "http://localhost:8080"
  timeout: "60s"

oidc:
  issuer: "https://example.com"
  client_id: "test-client"
  client_secret: "test-secret"
`
		configPath := test.TempFile(t, configContent)

		// Set environment variable to override auth mode
		test.SetEnv(t, "SMCP_CLIENT_AUTH_MODE", "oidc")

		// Load the config
		config, err := NewClientConfig(configPath)
		assert.NoError(t, err)
		assert.NotNil(t, config)

		// Verify auth mode is set to "oidc" from environment variable
		assert.Equal(t, OIDCAuthMode, config.Auth.Mode)
	})
}