package config

import (
	"os"
	"testing"

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
		tempFile, err := os.CreateTemp("", "client-config-*.yml")
		require.NoError(t, err)
		defer func() {
			err := os.Remove(tempFile.Name())
			if err != nil {
				t.Logf("Failed to remove temp file: %v", err)
			}
		}()
		
		_, err = tempFile.Write([]byte(configContent))
		require.NoError(t, err)
		require.NoError(t, tempFile.Close())
		
		configPath := tempFile.Name()

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
		tempFile, err := os.CreateTemp("", "client-config-*.yml")
		require.NoError(t, err)
		defer func() {
			err := os.Remove(tempFile.Name())
			if err != nil {
				t.Logf("Failed to remove temp file: %v", err)
			}
		}()
		
		_, err = tempFile.Write([]byte(configContent))
		require.NoError(t, err)
		require.NoError(t, tempFile.Close())
		
		configPath := tempFile.Name()
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
		tempFile, err = os.CreateTemp("", "client-config-*.yml")
		require.NoError(t, err)
		defer func() {
			err := os.Remove(tempFile.Name())
			if err != nil {
				t.Logf("Failed to remove temp file: %v", err)
			}
		}()
		
		_, err = tempFile.Write([]byte(configContent))
		require.NoError(t, err)
		require.NoError(t, tempFile.Close())
		
		configPath = tempFile.Name()
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
		tempFile, err = os.CreateTemp("", "client-config-*.yml")
		require.NoError(t, err)
		defer func() {
			err := os.Remove(tempFile.Name())
			if err != nil {
				t.Logf("Failed to remove temp file: %v", err)
			}
		}()
		
		_, err = tempFile.Write([]byte(configContent))
		require.NoError(t, err)
		require.NoError(t, tempFile.Close())
		
		configPath = tempFile.Name()
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
		tempFile, err := os.CreateTemp("", "client-config-*.yml")
		require.NoError(t, err)
		defer func() {
			err := os.Remove(tempFile.Name())
			if err != nil {
				t.Logf("Failed to remove temp file: %v", err)
			}
		}()
		
		_, err = tempFile.Write([]byte(configContent))
		require.NoError(t, err)
		require.NoError(t, tempFile.Close())
		
		configPath := tempFile.Name()

		// Save and restore environment variables
		oldEnv, exists := os.LookupEnv("SMCP_CLIENT_AUTH_MODE")
		defer func() {
			if exists {
				err := os.Setenv("SMCP_CLIENT_AUTH_MODE", oldEnv)
				if err != nil {
					t.Logf("Failed to restore environment variable: %v", err)
				}
			} else {
				err := os.Unsetenv("SMCP_CLIENT_AUTH_MODE")
				if err != nil {
					t.Logf("Failed to unset environment variable: %v", err)
				}
			}
		}()

		// Set environment variable to override auth mode
		err = os.Setenv("SMCP_CLIENT_AUTH_MODE", "oidc")
		require.NoError(t, err)

		// Load the config
		config, err := NewClientConfig(configPath)
		assert.NoError(t, err)
		assert.NotNil(t, config)

		// Verify auth mode is set to "oidc" from environment variable
		assert.Equal(t, OIDCAuthMode, config.Auth.Mode)
	})
}