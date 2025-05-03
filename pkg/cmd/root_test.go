package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootCmd(t *testing.T) {
	// Test that the root command is properly configured
	assert.Equal(t, "smcp-proxy", rootCmd.Use)
	assert.Contains(t, rootCmd.Short, "Secure MCP proxy")
	assert.Contains(t, rootCmd.Long, "reverse proxy server for MCP")
}

// This test helps verify that the root command can execute with help flag
func TestRootCommandExecution(t *testing.T) {
	// Create a buffer to capture output
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	
	// Set help flag to trigger help output without executing any actual commands
	rootCmd.SetArgs([]string{"--help"})
	
	// Execute the command
	err := rootCmd.Execute()
	assert.NoError(t, err)
	
	// Verify output contains expected help text
	output := buf.String()
	assert.Contains(t, output, "Usage:")
	assert.Contains(t, output, "Available Commands:")
}
