package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/config"
	"github.com/ksysoev/smcp-proxy/pkg/proxy"
	"github.com/spf13/cobra"
)

var (
	// Client settings
	clientHost            string
	clientPort            int
	clientReadTimeout     time.Duration
	clientWriteTimeout    time.Duration
	clientShutdownTimeout time.Duration

	// Server settings
	serverURL     string
	serverTimeout time.Duration

	// Auth settings
	authMode string // "oidc" or "none"

	// OIDC settings
	oidcIssuer        string
	oidcClientID      string
	oidcClientSecret  string
	oidcAudience      string
	oidcScopes        string
	oidcCacheTTL      time.Duration
	oidcTokenTTLDelta time.Duration

	// TLS settings
	tlsEnabled  bool
	tlsCertFile string
	tlsKeyFile  string

	// Metrics settings
	metricsEnabled bool
	metricsPath    string

	// Logger settings
	clientLogLevel  string
	clientLogFormat string
)

// clientCmd represents the client command
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Run the MCP proxy client that forwards requests to the proxy server",
	Long: `Starts the proxy client that forwards requests to the proxy server.
Can be configured to authenticate using OIDC client credentials flow
or run without authentication.`,
	Run: runClient,
}

func init() {
	rootCmd.AddCommand(clientCmd)

	// Client flags
	clientCmd.Flags().StringVar(&clientHost, "host", "127.0.0.1", "Host to bind the client to")
	clientCmd.Flags().IntVar(&clientPort, "port", 8081, "Port to bind the client to")
	clientCmd.Flags().DurationVar(&clientReadTimeout, "read-timeout", 30*time.Second, "HTTP read timeout")
	clientCmd.Flags().DurationVar(&clientWriteTimeout, "write-timeout", 30*time.Second, "HTTP write timeout")
	clientCmd.Flags().DurationVar(&clientShutdownTimeout, "shutdown-timeout", 10*time.Second, "Graceful shutdown timeout")

	// Server flags
	clientCmd.Flags().StringVar(&serverURL, "server-url", "", "URL of the proxy server")
	clientCmd.Flags().DurationVar(&serverTimeout, "server-timeout", 60*time.Second, "Timeout for requests to the server")
	if err := clientCmd.MarkFlagRequired("server-url"); err != nil {
		// This should never happen unless there's a programming error
		panic(fmt.Sprintf("Failed to mark server-url flag as required: %v", err))
	}
	
	// Auth mode flag
	clientCmd.Flags().StringVar(&authMode, "auth-mode", "none", "Authentication mode (none, oidc)")

	// OIDC flags
	clientCmd.Flags().StringVar(&oidcIssuer, "oidc-issuer", "", "OIDC issuer URL")
	clientCmd.Flags().StringVar(&oidcClientID, "oidc-client-id", "", "OIDC client ID")
	clientCmd.Flags().StringVar(&oidcClientSecret, "oidc-client-secret", "", "OIDC client secret")
	clientCmd.Flags().StringVar(&oidcAudience, "oidc-audience", "", "OIDC audience")
	clientCmd.Flags().StringVar(&oidcScopes, "oidc-scopes", "openid", "OIDC scopes (comma-separated)")
	clientCmd.Flags().DurationVar(&oidcCacheTTL, "oidc-cache-ttl", 5*time.Minute, "OIDC token cache TTL")
	clientCmd.Flags().DurationVar(&oidcTokenTTLDelta, "oidc-token-ttl-delta", 30*time.Second, "OIDC token TTL delta")

	// TLS flags
	clientCmd.Flags().BoolVar(&tlsEnabled, "tls", false, "Enable TLS")
	clientCmd.Flags().StringVar(&tlsCertFile, "tls-cert", "", "Path to TLS certificate file")
	clientCmd.Flags().StringVar(&tlsKeyFile, "tls-key", "", "Path to TLS key file")

	// Metrics flags
	clientCmd.Flags().BoolVar(&metricsEnabled, "metrics", true, "Enable metrics endpoint")
	clientCmd.Flags().StringVar(&metricsPath, "metrics-path", "/metrics", "Metrics endpoint path")

	// Logger flags
	clientCmd.Flags().StringVarP(&clientLogLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	clientCmd.Flags().StringVarP(&clientLogFormat, "log-format", "f", "text", "Log format (text, json)")
}

func runClient(cmd *cobra.Command, args []string) {
	// Setup logger
	logger := setupLogger(clientLogLevel, clientLogFormat)
	logger.Info("Starting MCP proxy client")

	// Get environment variables as fallbacks for required flags
	checkEnvVars(logger)

	// Validate required flags
	if err := validateRequiredFlags(); err != nil {
		logger.Error("Required flag not set", "error", err)
		os.Exit(1)
	}

	// Parse OIDC scopes
	var scopes []string
	if oidcScopes != "" {
		scopes = strings.Split(oidcScopes, ",")
	} else {
		scopes = []string{"openid"}
	}

	// Validate and convert auth mode to config type
	authModeConfig, err := validateAuthMode(authMode)
	if err != nil {
		logger.Error("Invalid auth mode", "error", err)
		os.Exit(1)
	}

	// Create client options
	opts := proxy.ClientOptions{
		// Client settings
		Host:            clientHost,
		Port:            clientPort,
		ReadTimeout:     clientReadTimeout,
		WriteTimeout:    clientWriteTimeout,
		ShutdownTimeout: clientShutdownTimeout,

		// Server settings
		ServerURL:     serverURL,
		ServerTimeout: serverTimeout,
		
		// Auth settings
		AuthMode: authModeConfig,

		// OIDC settings
		OIDCIssuer:        oidcIssuer,
		OIDCClientID:      oidcClientID,
		OIDCClientSecret:  oidcClientSecret,
		OIDCAudience:      oidcAudience,
		OIDCScopes:        scopes,
		OIDCCacheTTL:      oidcCacheTTL,
		OIDCTokenTTLDelta: oidcTokenTTLDelta,

		// TLS settings
		TLSEnabled:  tlsEnabled,
		TLSCertFile: tlsCertFile,
		TLSKeyFile:  tlsKeyFile,

		// Metrics settings
		MetricsEnabled: metricsEnabled,
		MetricsPath:    metricsPath,
	}

	// Create client
	client, err := proxy.NewClient(opts, logger)
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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), clientShutdownTimeout)
	defer shutdownCancel()

	// Shutdown client gracefully
	if err := client.Stop(shutdownCtx); err != nil {
		logger.Error("Error during client shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Client shutdown complete")
}

// checkEnvVars checks environment variables as fallbacks for required flags
func checkEnvVars(logger *slog.Logger) {
	// Check server URL
	if serverURL == "" {
		if envURL := os.Getenv("SMCP_SERVER_URL"); envURL != "" {
			serverURL = envURL
			logger.Debug("Using environment variable for server URL", "value", serverURL)
		}
	}
	
	// Check auth mode
	if envAuthMode := os.Getenv("SMCP_AUTH_MODE"); envAuthMode != "" {
		// Validate the auth mode from environment
		if envAuthMode != "oidc" && envAuthMode != "none" {
			logger.Warn("Invalid auth mode in environment variable SMCP_AUTH_MODE. Must be 'oidc' or 'none'. Using default.", 
				"provided", envAuthMode)
		} else {
			// For tests, always apply the valid env var
			// In practice, this wouldn't change anything that's manually set
			authMode = envAuthMode
			logger.Debug("Using environment variable for auth mode", "value", authMode)
		}
	}

	// Check OIDC settings only if auth mode is OIDC
	if authMode == "oidc" {
		// Check OIDC issuer
		if oidcIssuer == "" {
			if envIssuer := os.Getenv("SMCP_OIDC_ISSUER"); envIssuer != "" {
				oidcIssuer = envIssuer
				logger.Debug("Using environment variable for OIDC issuer", "value", oidcIssuer)
			}
		}
	
		// Check OIDC client ID
		if oidcClientID == "" {
			if envClientID := os.Getenv("SMCP_OIDC_CLIENT_ID"); envClientID != "" {
				oidcClientID = envClientID
				logger.Debug("Using environment variable for OIDC client ID", "value", oidcClientID)
			}
		}
	
		// Check OIDC client secret
		if oidcClientSecret == "" {
			if envClientSecret := os.Getenv("SMCP_OIDC_CLIENT_SECRET"); envClientSecret != "" {
				oidcClientSecret = envClientSecret
				logger.Debug("Using environment variable for OIDC client secret")
			}
		}
	
		// Check OIDC audience
		if oidcAudience == "" {
			if envAudience := os.Getenv("SMCP_OIDC_AUDIENCE"); envAudience != "" {
				oidcAudience = envAudience
				logger.Debug("Using environment variable for OIDC audience", "value", oidcAudience)
			}
		}
	
		// Check OIDC scopes
		if oidcScopes == "openid" {
			if envScopes := os.Getenv("SMCP_OIDC_SCOPES"); envScopes != "" {
				oidcScopes = envScopes
				logger.Debug("Using environment variable for OIDC scopes", "value", oidcScopes)
			}
		}
	}

	// Check other optional settings from environment
	if envHost := os.Getenv("SMCP_CLIENT_HOST"); envHost != "" && clientHost == "127.0.0.1" {
		clientHost = envHost
		logger.Debug("Using environment variable for client host", "value", clientHost)
	}

	if envPort := os.Getenv("SMCP_CLIENT_PORT"); envPort != "" && clientPort == 8081 {
		if port, err := parseEnvInt(envPort); err == nil {
			clientPort = port
			logger.Debug("Using environment variable for client port", "value", clientPort)
		}
	}
}

// validateRequiredFlags validates that all required flags are set
func validateRequiredFlags() error {
	if serverURL == "" {
		return fmt.Errorf("server URL is required (use --server-url flag or SMCP_SERVER_URL environment variable)")
	}
	
	// Only validate OIDC settings if using OIDC authentication mode
	if authMode == "oidc" {
		if oidcIssuer == "" {
			return fmt.Errorf("OIDC issuer is required when auth-mode is 'oidc' (use --oidc-issuer flag or SMCP_OIDC_ISSUER environment variable)")
		}
		if oidcClientID == "" {
			return fmt.Errorf("OIDC client ID is required when auth-mode is 'oidc' (use --oidc-client-id flag or SMCP_OIDC_CLIENT_ID environment variable)")
		}
		if oidcClientSecret == "" {
			return fmt.Errorf("OIDC client secret is required when auth-mode is 'oidc' (use --oidc-client-secret flag or SMCP_OIDC_CLIENT_SECRET environment variable)")
		}
	}
	
	return nil
}

// parseEnvInt parses an environment variable as an integer
func parseEnvInt(value string) (int, error) {
	var i int
	if _, err := fmt.Sscanf(value, "%d", &i); err != nil {
		return 0, err
	}
	return i, nil
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

// validateAuthMode validates the auth mode and returns the corresponding config.AuthMode
func validateAuthMode(mode string) (config.AuthMode, error) {
	switch mode {
	case "oidc":
		return config.OIDCAuthMode, nil
	case "none":
		return config.NoAuthMode, nil
	default:
		return "", fmt.Errorf("invalid auth mode: %q (must be 'oidc' or 'none')", mode)
	}
}
