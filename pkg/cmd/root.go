package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "smcp-proxy",
	Short: "Secure MCP proxy with OIDC authentication",
	Long: `A reverse proxy server for MCP that provides authentication and authorization 
of client requests via OIDC. It supports both server and client components.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
