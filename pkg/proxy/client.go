package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/auth"
	"github.com/ksysoev/smcp-proxy/pkg/config"
)

// Client represents the proxy client
type Client struct {
	httpServer  *http.Server
	logger      *slog.Logger
	tokenClient auth.TokenClient
	cfg         *config.ClientConfig
	mux         *http.ServeMux
}

// clientTransport is a transport that logs request details
type clientTransport struct {
	base   http.RoundTripper
	logger *slog.Logger
}

// RoundTrip implements the http.RoundTripper interface
func (t *clientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	t.logger.Debug("Sending request", "method", req.Method, "url", req.URL.String())

	resp, err := t.base.RoundTrip(req)

	if err != nil {
		t.logger.Error("Request failed", "method", req.Method, "url", req.URL.String(), "error", err, "duration", time.Since(start))
		return nil, err
	}

	t.logger.Debug("Received response",
		"method", req.Method,
		"url", req.URL.String(),
		"status", resp.StatusCode,
		"duration", time.Since(start),
	)

	return resp, nil
}

// ClientOptions holds options for creating a new client
type ClientOptions struct {
	AuthMode          config.AuthMode
	MetricsPath       string
	TLSKeyFile        string
	TLSCertFile       string
	Host              string
	ServerURL         string
	OIDCIssuer        string
	OIDCClientID      string
	OIDCClientSecret  string
	OIDCAudience      string
	OIDCScopes        []string
	ShutdownTimeout   time.Duration
	OIDCCacheTTL      time.Duration
	OIDCTokenTTLDelta time.Duration
	WriteTimeout      time.Duration
	ReadTimeout       time.Duration
	Port              int
	ServerTimeout     time.Duration
	TLSEnabled        bool
	MetricsEnabled    bool
}

// NewClient creates a new proxy client
func NewClient(
	opts ClientOptions,
	logger *slog.Logger,
) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Create the appropriate token client based on auth mode
	var tokenClient auth.TokenClient
	if opts.AuthMode == config.OIDCAuthMode {
		// Create OIDC token client
		tokenClient = auth.NewOIDCTokenClient(
			opts.OIDCIssuer,
			opts.OIDCClientID,
			opts.OIDCClientSecret,
			opts.OIDCAudience,
			opts.OIDCScopes,
			opts.OIDCCacheTTL,
			opts.OIDCTokenTTLDelta,
			logger,
		)
	} else {
		// Create no-op token client for no-auth mode
		tokenClient = auth.NewNoAuthTokenClient()
	}

	// Create server mux
	mux := http.NewServeMux()

	// Setup HTTP server
	addr := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
	}

	// Create a config struct for backward compatibility with other methods
	cfg := &config.ClientConfig{}
	cfg.Server.URL = opts.ServerURL
	cfg.Server.Timeout = opts.ServerTimeout
	cfg.TLS.Enabled = opts.TLSEnabled
	cfg.TLS.CertFile = opts.TLSCertFile
	cfg.TLS.KeyFile = opts.TLSKeyFile
	cfg.Metrics.Enabled = opts.MetricsEnabled
	cfg.Metrics.Path = opts.MetricsPath
	cfg.Client.ShutdownTimeout = opts.ShutdownTimeout

	client := &Client{
		httpServer:  httpServer,
		logger:      logger,
		tokenClient: tokenClient,
		cfg:         cfg,
		mux:         mux,
	}

	// Initialize routes
	client.initRoutes()

	return client, nil
}

// Start starts the proxy client
func (c *Client) Start() error {
	c.logger.Info("Starting client", "address", c.httpServer.Addr)

	// Start HTTP server
	if c.cfg.TLS.Enabled {
		return c.httpServer.ListenAndServeTLS(c.cfg.TLS.CertFile, c.cfg.TLS.KeyFile)
	}
	return c.httpServer.ListenAndServe()
}

// Stop gracefully stops the proxy client
func (c *Client) Stop(ctx context.Context) error {
	c.logger.Info("Stopping client")
	return c.httpServer.Shutdown(ctx)
}

// initRoutes initializes the client routes
func (c *Client) initRoutes() {
	// Add health check endpoint
	c.mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			c.logger.Error("Failed to write health check response", "error", err)
		}
	})

	// Add metrics endpoint if enabled
	if c.cfg.Metrics.Enabled {
		c.mux.HandleFunc("GET "+c.cfg.Metrics.Path, func(w http.ResponseWriter, r *http.Request) {
			// Metrics implementation would go here
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("Metrics would be here"))
			if err != nil {
				c.logger.Error("Failed to write metrics response", "error", err)
			}
		})
	}

	// Create server proxy
	targetURL, err := url.Parse(c.cfg.Server.URL)
	if err != nil {
		c.logger.Error("Failed to parse target URL", "url", c.cfg.Server.URL, "error", err)
		return
	}

	// Create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	var transport http.RoundTripper

	// Create appropriate transport based on auth mode
	if c.cfg.Auth.Mode == config.OIDCAuthMode {
		// Create transport with token authentication for OIDC mode
		tokenTransport := &auth.TokenTransport{
			Base:        http.DefaultTransport,
			Client:      c.tokenClient,
			CacheErrors: true,
			Logger:      c.logger,
		}
		transport = tokenTransport
	} else {
		// Create no-auth transport for none mode
		noAuthTransport := &auth.NoAuthTransport{
			Base:   http.DefaultTransport,
			Logger: c.logger,
		}
		transport = noAuthTransport
	}

	// Create instrumented transport wrapping the appropriate transport
	proxy.Transport = &clientTransport{
		base:   transport,
		logger: c.logger.With("target", c.cfg.Server.URL),
	}

	// Setup error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		c.logger.Error("Proxy error", "error", err, "path", r.URL.Path)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	}

	// Register the proxy handler for all paths
	c.mux.Handle("* /", proxy)
}
