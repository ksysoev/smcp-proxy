package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestRootCmd(t *testing.T) {
	// Test that the root command is properly configured
	assert.Equal(t, "smcp-proxy", rootCmd.Use)
	assert.Contains(t, rootCmd.Short, "Secure MCP proxy")
	assert.Contains(t, rootCmd.Long, "reverse proxy server for MCP")
}

// This is a helper to test cobra commands
func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}
