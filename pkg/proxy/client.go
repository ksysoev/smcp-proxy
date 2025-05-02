package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

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

// NewClient creates a new proxy client
func NewClient(
	cfg *config.ClientConfig,
	logger *slog.Logger,
) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Create the token client
	tokenClient := auth.NewOIDCTokenClient(
		cfg.OIDC.Issuer,
		cfg.OIDC.ClientID,
		cfg.OIDC.ClientSecret,
		cfg.OIDC.Audience,
		cfg.OIDC.Scopes,
		cfg.OIDC.CacheTTL,
		cfg.OIDC.TokenTTLDelta,
		logger,
	)

	// Create server mux
	mux := http.NewServeMux()

	// Setup HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Client.Host, cfg.Client.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  cfg.Client.ReadTimeout,
		WriteTimeout: cfg.Client.WriteTimeout,
	}

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
		w.Write([]byte("OK"))
	})

	// Add metrics endpoint if enabled
	if c.cfg.Metrics.Enabled {
		c.mux.HandleFunc("GET "+c.cfg.Metrics.Path, func(w http.ResponseWriter, r *http.Request) {
			// Metrics implementation would go here
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Metrics would be here"))
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

	// Create transport with token authentication
	tokenTransport := &auth.TokenTransport{
		Base:        http.DefaultTransport,
		Client:      c.tokenClient,
		CacheErrors: true,
		Logger:      c.logger,
	}

	// Create instrumented transport wrapping the token transport
	proxy.Transport = &instrumentedTransport{
		base:   tokenTransport,
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
