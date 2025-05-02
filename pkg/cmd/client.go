package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ksysoev/smcp-proxy/pkg/config"
	"github.com/ksysoev/smcp-proxy/pkg/proxy"
	"github.com/spf13/cobra"
)

var (
	clientConfigFile string
	clientLogLevel   string
	clientLogFormat  string
)

// clientCmd represents the client command
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Run the MCP proxy client that authenticates to the proxy server",
	Long: `Starts the proxy client that implements the client credentials flow
to acquire OIDC tokens and forwards requests to the proxy server.`,
	Run: runClient,
}

func init() {
	rootCmd.AddCommand(clientCmd)

	// Add flags
	clientCmd.Flags().StringVarP(&clientConfigFile, "config", "c", "configs/proxy-client.yml", "Path to the configuration file")
	clientCmd.Flags().StringVarP(&clientLogLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	clientCmd.Flags().StringVarP(&clientLogFormat, "log-format", "f", "text", "Log format (text, json)")
}

func runClient(cmd *cobra.Command, args []string) {
	// Setup logger
	logger := setupLogger(clientLogLevel, clientLogFormat)
	logger.Info("Starting MCP proxy client")

	// Load configuration
	cfg, err := config.NewClientConfig(clientConfigFile)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create client
	client, err := proxy.NewClient(cfg, logger)
	if err != nil {
		logger.Error("Failed to create client", "error", err)
		os.Exit(1)
	}

	// Start client in a goroutine
	go func() {
		if err := client.Start(); err != nil {
			logger.Error("Client error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Client.ShutdownTimeout)
	defer shutdownCancel()

	// Shutdown client gracefully
	if err := client.Stop(shutdownCtx); err != nil {
		logger.Error("Error during client shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Client shutdown complete")
}

// setupLogger creates a new logger with the specified level and format
func setupLogger(level, format string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	if strings.ToLower(format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
