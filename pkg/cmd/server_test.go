package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerCommand(t *testing.T) {
	// Test that the server command is properly configured
	assert.Equal(t, "server", serverCmd.Use)
	assert.Contains(t, serverCmd.Short, "MCP proxy server")
	assert.Contains(t, serverCmd.Long, "forwards requests to the configured MCP servers")
	assert.NotNil(t, serverCmd.Run)
}

func TestServerFlagSetup(t *testing.T) {
	// Test all flags are registered with correct defaults
	cmd := serverCmd

	// Config file flag
	configFlag := cmd.Flag("config")
	assert.NotNil(t, configFlag)
	assert.Equal(t, "configs/proxy-server.yml", configFlag.Value.String())

	// Log level flag
	logLevelFlag := cmd.Flag("log-level")
	assert.NotNil(t, logLevelFlag)
	assert.Equal(t, "info", logLevelFlag.Value.String())

	// Log format flag
	logFormatFlag := cmd.Flag("log-format")
	assert.NotNil(t, logFormatFlag)
	assert.Equal(t, "text", logFormatFlag.Value.String())
	
	// Auth mode flag
	authModeFlag := cmd.Flag("auth-mode")
	assert.NotNil(t, authModeFlag)
	assert.Equal(t, "none", authModeFlag.Value.String())
}

func TestServerLoggerSetup(t *testing.T) {
	// Test that the server uses the same logger setup function as the client
	t.Run("Default level and format", func(t *testing.T) {
		logger := setupLogger("info", "text")
		assert.NotNil(t, logger)
	})

	t.Run("Custom level and format", func(t *testing.T) {
		logger := setupLogger("debug", "json")
		assert.NotNil(t, logger)
	})
}

// This test requires mocking the config loading and server creation
// We'll create a simple test that verifies the command doesn't panic with proper args
func TestServerCommandExecution(t *testing.T) {
	// Skip this test if running in CI or if we want to avoid file system operations
	if os.Getenv("CI") != "" || os.Getenv("SKIP_FS_TESTS") != "" {
		t.Skip("Skipping test that requires file system access")
	}

	// Create a temporary config file to avoid failure
	tempFile, err := os.CreateTemp("", "server-config-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := os.Remove(tempFile.Name())
		if err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()

	// Write a minimal valid config
	minimalConfig := `
server:
  host: "127.0.0.1"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  shutdown_timeout: 10s

validator:
  jwks_url: "https://example.com/.well-known/jwks.json"
  audience: "test-audience"

backends:
  default:
    type: "http"
    url: "http://example.com/api"
`
	if _, err := tempFile.Write([]byte(minimalConfig)); err != nil {
		t.Fatal(err)
	}
	if err := tempFile.Close(); err != nil {
		t.Fatal(err)
	}

	// Save original value and restore after test
	origConfigFile := serverConfigFile
	defer func() {
		serverConfigFile = origConfigFile
	}()

	// Set the config file to our temp file
	serverConfigFile = tempFile.Name()

	// This is a simple check that at least the flag gets processed correctly
	// We're not testing the actual execution since that would require more complex mocking
	f := serverCmd.Flag("config")
	assert.NotNil(t, f)
	assert.Equal(t, tempFile.Name(), serverConfigFile)
}
