package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// MCPBackend defines configuration for a single MCP backend server
type MCPBackend struct {
	// Name is a unique identifier for this backend
	Name string `mapstructure:"name"`
	// URL is the backend server URL
	URL string `mapstructure:"url"`
	// Path is the URL path prefix to match for this backend (e.g., "/api/v1")
	Path string `mapstructure:"path"`
	// StripPath determines whether to strip the path prefix before forwarding
	StripPath bool `mapstructure:"strip_path"`
	// Timeout overrides the global timeout for this backend
	Timeout time.Duration `mapstructure:"timeout"`
}

// ServerConfig holds the configuration for the proxy server
type ServerConfig struct {
	// Server configuration
	Server struct {
		Host            string        `mapstructure:"host"`
		Port            int           `mapstructure:"port"`
		ReadTimeout     time.Duration `mapstructure:"read_timeout"`
		WriteTimeout    time.Duration `mapstructure:"write_timeout"`
		ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	} `mapstructure:"server"`

	// MCP upstream configuration
	MCP struct {
		// Backends defines multiple MCP backends with their own paths
		Backends []MCPBackend   `mapstructure:"backends"`
		// Global timeout for all MCP backends
		Timeout  time.Duration  `mapstructure:"timeout"`
		// Legacy endpoints configuration (deprecated)
		Endpoints []string      `mapstructure:"endpoints"`
	} `mapstructure:"mcp"`

	// OIDC configuration
	OIDC struct {
		Issuers []string `mapstructure:"issuers"`
		// Required claims that must be present in the token
		RequiredClaims map[string]string `mapstructure:"required_claims"`
		// Optional claims that if present must match the specified values
		OptionalClaims map[string]string `mapstructure:"optional_claims"`
		// Audience that tokens must be issued for
		Audience string `mapstructure:"audience"`
	} `mapstructure:"oidc"`

	// TLS configuration
	TLS struct {
		Enabled  bool   `mapstructure:"enabled"`
		CertFile string `mapstructure:"cert_file"`
		KeyFile  string `mapstructure:"key_file"`
	} `mapstructure:"tls"`

	// Metrics configuration
	Metrics struct {
		Enabled bool   `mapstructure:"enabled"`
		Path    string `mapstructure:"path"`
	} `mapstructure:"metrics"`
}

// NewServerConfig creates a new server configuration
func NewServerConfig(configPath string) (*ServerConfig, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Set default values
	setServerDefaults(v)

	// Read configuration file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Override config values from environment variables
	v.SetEnvPrefix("SMCP_PROXY")
	v.AutomaticEnv()

	var config ServerConfig
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// setServerDefaults sets default values for the server configuration
func setServerDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.shutdown_timeout", "10s")

	// MCP defaults
	v.SetDefault("mcp.timeout", "60s")

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.path", "/metrics")

	// TLS defaults
	v.SetDefault("tls.enabled", false)
}