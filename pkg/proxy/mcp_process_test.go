package proxy

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/config"
	"github.com/ksysoev/smcp-proxy/pkg/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPProcess(t *testing.T) {
	logger := test.NewTestLogger()

	// Skip on Windows as echo behaves differently
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows")
	}

	t.Run("Start and stop process", func(t *testing.T) {
		// Create a configuration for a simple echo process
		cfg := config.StdioConfig{
			Command:      "echo",
			Args:         []string{"Hello"},
			StdioTimeout: 5 * time.Second,
		}

		// Create the process manager
		process := NewMCPProcess("test-process", cfg, logger)

		// Start the process
		err := process.Start()
		require.NoError(t, err)

		// Verify the process was started
		assert.True(t, process.started)
		assert.NotNil(t, process.cmd)
		assert.NotNil(t, process.stdin)
		assert.NotNil(t, process.stdout)

		// Stop the process
		err = process.Stop()
		require.NoError(t, err)

		// Verify the process was stopped
		assert.False(t, process.started)
	})

	t.Run("Start process with working directory and environment", func(t *testing.T) {
		// Create a temporary directory for the working directory
		tempDir, err := os.MkdirTemp("", "mcp-process-test-*")
		require.NoError(t, err)
		defer func() {
			if err := os.RemoveAll(tempDir); err != nil {
				t.Logf("Failed to remove temporary directory: %v", err)
			}
		}()

		// Create a configuration with working directory and environment
		cfg := config.StdioConfig{
			Command:    "env",
			WorkingDir: tempDir,
			Env: map[string]string{
				"TEST_VAR": "test-value",
			},
			StdioTimeout: 5 * time.Second,
		}

		// Create the process manager
		process := NewMCPProcess("test-env", cfg, logger)

		// Start the process
		err = process.Start()
		require.NoError(t, err)

		// Verify the working directory was set
		assert.Equal(t, tempDir, process.cmd.Dir)

		// Stop the process
		err = process.Stop()
		require.NoError(t, err)
	})

	t.Run("Process request with cat", func(t *testing.T) {
		// Create a configuration for a cat process (reads input and echoes it back)
		cfg := config.StdioConfig{
			Command:      "cat",
			StdioTimeout: 5 * time.Second,
		}

		// Create the process manager
		process := NewMCPProcess("test-request", cfg, logger)

		// Start the process
		err := process.Start()
		require.NoError(t, err)
		defer func() {
			if err := process.Stop(); err != nil {
				t.Logf("Failed to stop process: %v", err)
			}
		}()

		// Create a request input
		input := map[string]interface{}{
			"message": "Hello, world!",
			"number":  42,
		}

		// Create a context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Send the request
		response, err := process.Request(ctx, input)
		require.NoError(t, err)

		// Verify the response contains the same data we sent (cat echoes back the input)
		assert.Equal(t, "Hello, world!", response["message"])
		assert.Equal(t, float64(42), response["number"])

		// There should also be request_id and timestamp added
		assert.Contains(t, response, "request_id")
		assert.Contains(t, response, "timestamp")
	})

	t.Run("Process request timeout", func(t *testing.T) {
		// Use the sleep command to simulate a slow process
		cfg := config.StdioConfig{
			Command:      "sleep",
			Args:         []string{"10"},  // Sleep for 10 seconds
			StdioTimeout: 1 * time.Second, // But timeout after 1 second
		}

		// Create the process manager
		process := NewMCPProcess("test-timeout", cfg, logger)

		// Start the process
		err := process.Start()
		require.NoError(t, err)
		defer func() {
			if err := process.Stop(); err != nil {
				t.Logf("Failed to stop process: %v", err)
			}
		}()

		// Create a context with a short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Send a request (should timeout)
		input := map[string]interface{}{
			"message": "This should timeout",
		}

		// The request should fail due to timeout
		_, err = process.Request(ctx, input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})

	t.Run("Start process with invalid command", func(t *testing.T) {
		// Create a configuration with a non-existent command
		cfg := config.StdioConfig{
			Command: "non-existent-command",
		}

		// Create the process manager
		process := NewMCPProcess("test-invalid", cfg, logger)

		// Start should fail
		err := process.Start()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to start process")

		// Process should not be marked as started
		assert.False(t, process.started)
	})
}
