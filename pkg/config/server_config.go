package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// TransportType represents the type of transport to use for the MCP backend
type TransportType string

const (
	// HTTPTransport represents an HTTP transport for remote MCP servers
	HTTPTransport TransportType = "http"
	// StdioTransport represents a stdio transport for local MCP servers
	StdioTransport TransportType = "stdio"
)

// StdioConfig holds configuration for a stdio transport
type StdioConfig struct {
	// Command is the command to execute for the local MCP server
	Command string `mapstructure:"command"`
	// Args are the arguments to pass to the command
	Args []string `mapstructure:"args"`
	// WorkingDir is the working directory for the command
	WorkingDir string `mapstructure:"working_dir"`
	// Env is a map of environment variables to set for the command
	Env map[string]string `mapstructure:"env"`
	// StdioTimeout is the timeout for stdio operations
	StdioTimeout time.Duration `mapstructure:"stdio_timeout"`
}

// MCPBackend defines configuration for a single MCP backend server
type MCPBackend struct {
	// ID is a unique identifier for this backend
	ID string `mapstructure:"id"`
	// Name is a human-readable name for this backend
	Name string `mapstructure:"name"`
	// Transport is the type of transport to use (http or stdio)
	Transport TransportType `mapstructure:"transport"`
	
	// HTTP Transport specific fields
	URL string `mapstructure:"url"`
	
	// Stdio Transport specific fields
	Stdio StdioConfig `mapstructure:"stdio"`
	
	// Common fields
	// Path is the URL path prefix to match for this backend (e.g., "/api/v1")
	Path string `mapstructure:"path"`
	// StripPath determines whether to strip the path prefix before forwarding
	StripPath bool `mapstructure:"strip_path"`
	// Timeout overrides the global timeout for this backend
	Timeout time.Duration `mapstructure:"timeout"`
	// Model is the Anthropic model associated with this backend (e.g., "claude-3-opus-20240229")
	Model string `mapstructure:"model"`
	// MaxTokens is the maximum number of tokens for this model
	MaxTokens int `mapstructure:"max_tokens"`
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