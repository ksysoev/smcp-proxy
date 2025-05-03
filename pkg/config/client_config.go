package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// ClientConfig holds the configuration for the proxy client
type ClientConfig struct {
	TLS struct {
		CertFile string `mapstructure:"cert_file"`
		KeyFile  string `mapstructure:"key_file"`
		Enabled  bool   `mapstructure:"enabled"`
	} `mapstructure:"tls"`
	Metrics struct {
		Path    string `mapstructure:"path"`
		Enabled bool   `mapstructure:"enabled"`
	} `mapstructure:"metrics"`
	Server struct {
		URL     string        `mapstructure:"url"`
		Timeout time.Duration `mapstructure:"timeout"`
	} `mapstructure:"server"`
	Auth struct {
		Mode AuthMode `mapstructure:"mode"`
	} `mapstructure:"auth"`
	OIDC struct {
		Issuer        string        `mapstructure:"issuer"`
		ClientID      string        `mapstructure:"client_id"`
		ClientSecret  string        `mapstructure:"client_secret"`
		Audience      string        `mapstructure:"audience"`
		Scopes        []string      `mapstructure:"scopes"`
		CacheTTL      time.Duration `mapstructure:"cache_ttl"`
		TokenTTLDelta time.Duration `mapstructure:"token_ttl_delta"`
	} `mapstructure:"oidc"`
	Client struct {
		Host            string        `mapstructure:"host"`
		Port            int           `mapstructure:"port"`
		ReadTimeout     time.Duration `mapstructure:"read_timeout"`
		WriteTimeout    time.Duration `mapstructure:"write_timeout"`
		ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	} `mapstructure:"client"`
}

// NewClientConfig creates a new client configuration
func NewClientConfig(configPath string) (*ClientConfig, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Set default values
	setClientDefaults(v)

	// Read configuration file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Override config values from environment variables
	v.SetEnvPrefix("SMCP_CLIENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var config ClientConfig
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate required configuration
	if err := validateClientConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// setClientDefaults sets default values for the client configuration
func setClientDefaults(v *viper.Viper) {
	// Client defaults
	v.SetDefault("client.host", "127.0.0.1")
	v.SetDefault("client.port", 8081)
	v.SetDefault("client.read_timeout", "30s")
	v.SetDefault("client.write_timeout", "30s")
	v.SetDefault("client.shutdown_timeout", "10s")

	// Auth defaults - no authentication by default
	v.SetDefault("auth.mode", string(NoAuthMode))

	// Server defaults
	v.SetDefault("server.timeout", "60s")

	// OIDC defaults
	v.SetDefault("oidc.scopes", []string{"openid"})
	v.SetDefault("oidc.cache_ttl", "5m")
	v.SetDefault("oidc.token_ttl_delta", "30s") // Refresh token 30s before it expires

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.path", "/metrics")

	// TLS defaults
	v.SetDefault("tls.enabled", false)
}

// validateClientConfig validates the client configuration
func validateClientConfig(config *ClientConfig) error {
	// Validate server URL
	if config.Server.URL == "" {
		return fmt.Errorf("server URL is required")
	}

	// Only validate OIDC configuration if OIDC auth mode is enabled
	if config.Auth.Mode == OIDCAuthMode {
		// Validate required OIDC configuration
		if config.OIDC.Issuer == "" {
			return fmt.Errorf("OIDC issuer is required when auth.mode is 'oidc'")
		}
		if config.OIDC.ClientID == "" {
			return fmt.Errorf("OIDC client ID is required when auth.mode is 'oidc'")
		}
		if config.OIDC.ClientSecret == "" {
			return fmt.Errorf("OIDC client secret is required when auth.mode is 'oidc'")
		}
	}

	return nil
}
