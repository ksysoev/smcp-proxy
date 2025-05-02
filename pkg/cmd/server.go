package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/config"
	"github.com/ksysoev/smcp-proxy/pkg/proxy"
	"github.com/spf13/cobra"
)

var (
	serverConfigFile string
	serverLogLevel   string
	serverLogFormat  string
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run the MCP proxy server with OIDC authentication",
	Long: `Starts the proxy server that validates OIDC tokens and forwards 
authenticated requests to the configured MCP servers.`,
	Run: runServer,
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// Add flags
	serverCmd.Flags().StringVarP(&serverConfigFile, "config", "c", "configs/proxy-server.yml", "Path to the configuration file")
	serverCmd.Flags().StringVarP(&serverLogLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	serverCmd.Flags().StringVarP(&serverLogFormat, "log-format", "f", "text", "Log format (text, json)")
}

func runServer(cmd *cobra.Command, args []string) {
	// Setup logger
	logger := setupLogger(serverLogLevel, serverLogFormat)
	logger.Info("Starting MCP proxy server")

	// Load configuration
	cfg, err := config.NewServerConfig(serverConfigFile)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create application context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create server
	server, err := proxy.NewServer(ctx, cfg, logger)
	if err != nil {
		logger.Error("Failed to create server", "error", err)
		os.Exit(1)
	}

	// Start server in a goroutine
	go func() {
		if err := server.Start(); err != nil {
			logger.Error("Server error", "error", err)
			cancel()
		}
	}()

	// Wait for interrupt signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal or context cancellation
	select {
	case <-sig:
		logger.Info("Received shutdown signal")
	case <-ctx.Done():
		logger.Info("Context cancelled")
	}

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	// Shutdown server gracefully
	if err := server.Stop(shutdownCtx); err != nil {
		logger.Error("Error during server shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Server shutdown complete")
}
