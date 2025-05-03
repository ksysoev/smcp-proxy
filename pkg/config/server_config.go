package config

import (
	"fmt"
	"strings"
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
	Env          map[string]string `mapstructure:"env"`
	Command      string            `mapstructure:"command"`
	WorkingDir   string            `mapstructure:"working_dir"`
	Args         []string          `mapstructure:"args"`
	StdioTimeout time.Duration     `mapstructure:"stdio_timeout"`
}

// MCPBackend defines configuration for a single MCP backend server
type MCPBackend struct {
	ID        string        `mapstructure:"id"`
	Name      string        `mapstructure:"name"`
	Transport TransportType `mapstructure:"transport"`
	URL       string        `mapstructure:"url"`
	Path      string        `mapstructure:"path"`
	Model     string        `mapstructure:"model"`
	Stdio     StdioConfig   `mapstructure:"stdio"`
	Timeout   time.Duration `mapstructure:"timeout"`
	MaxTokens int           `mapstructure:"max_tokens"`
	StripPath bool          `mapstructure:"strip_path"`
}

// AuthMode defines the authentication mode
type AuthMode string

const (
	// OIDCAuthMode uses OIDC authentication
	OIDCAuthMode AuthMode = "oidc"
	// NoAuthMode disables authentication
	NoAuthMode AuthMode = "none"
)

// ServerConfig holds the configuration for the proxy server
type ServerConfig struct {
	Auth struct {
		Mode AuthMode `mapstructure:"mode"`
	} `mapstructure:"auth"`
	OIDC struct {
		RequiredClaims map[string]string `mapstructure:"required_claims"`
		OptionalClaims map[string]string `mapstructure:"optional_claims"`
		Audience       string            `mapstructure:"audience"`
		Issuers        []string          `mapstructure:"issuers"`
	} `mapstructure:"oidc"`
	TLS struct {
		CertFile string `mapstructure:"cert_file"`
		KeyFile  string `mapstructure:"key_file"`
		Enabled  bool   `mapstructure:"enabled"`
	} `mapstructure:"tls"`
	Metrics struct {
		Path    string `mapstructure:"path"`
		Enabled bool   `mapstructure:"enabled"`
	} `mapstructure:"metrics"`
	MCP struct {
		Backends  []MCPBackend  `mapstructure:"backends"`
		Endpoints []string      `mapstructure:"endpoints"`
		Timeout   time.Duration `mapstructure:"timeout"`
	} `mapstructure:"mcp"`
	Server struct {
		Host            string        `mapstructure:"host"`
		Port            int           `mapstructure:"port"`
		ReadTimeout     time.Duration `mapstructure:"read_timeout"`
		WriteTimeout    time.Duration `mapstructure:"write_timeout"`
		ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	} `mapstructure:"server"`
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
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
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

	// Auth defaults - no authentication by default
	v.SetDefault("auth.mode", string(NoAuthMode))

	// MCP defaults
	v.SetDefault("mcp.timeout", "60s")

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.path", "/metrics")

	// TLS defaults
	v.SetDefault("tls.enabled", false)
}
