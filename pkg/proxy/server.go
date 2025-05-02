package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/auth"
	"github.com/ksysoev/smcp-proxy/pkg/config"
)

// Server represents the proxy server
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
	validator  auth.TokenValidator
	cfg        *config.ServerConfig
	mux        *http.ServeMux
}

// NewServer creates a new proxy server
func NewServer(
	ctx context.Context,
	cfg *config.ServerConfig,
	logger *slog.Logger,
) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Create the token validator
	validator, err := auth.NewOIDCTokenValidator(
		ctx,
		cfg.OIDC.Issuers,
		cfg.OIDC.Audience,
		cfg.OIDC.RequiredClaims,
		cfg.OIDC.OptionalClaims,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create token validator: %w", err)
	}

	// Create server
	mux := http.NewServeMux()

	// Setup HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	server := &Server{
		httpServer: httpServer,
		logger:     logger,
		validator:  validator,
		cfg:        cfg,
		mux:        mux,
	}

	// Initialize routes
	server.initRoutes()

	return server, nil
}

// Start starts the proxy server
func (s *Server) Start() error {
	s.logger.Info("Starting server", "address", s.httpServer.Addr)

	// Start HTTP server
	if s.cfg.TLS.Enabled {
		return s.httpServer.ListenAndServeTLS(s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile)
	}
	return s.httpServer.ListenAndServe()
}

// Stop gracefully stops the proxy server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping server")
	return s.httpServer.Shutdown(ctx)
}

// initRoutes initializes the server routes
func (s *Server) initRoutes() {
	// Add health check endpoint
	s.mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Add metrics endpoint if enabled
	if s.cfg.Metrics.Enabled {
		s.mux.HandleFunc("GET " + s.cfg.Metrics.Path, func(w http.ResponseWriter, r *http.Request) {
			// Metrics implementation would go here
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Metrics would be here"))
		})
	}

	// Handle legacy endpoints configuration (for backward compatibility)
	if len(s.cfg.MCP.Endpoints) > 0 && len(s.cfg.MCP.Backends) == 0 {
		s.logger.Warn("Using deprecated 'endpoints' configuration. Please migrate to 'backends' configuration.")
		// Create a root backend for each endpoint
		for _, endpoint := range s.cfg.MCP.Endpoints {
			s.setupMCPProxy(&config.MCPBackend{
				Name:      "legacy",
				URL:       endpoint,
				Path:      "/",
				StripPath: false,
				Timeout:   s.cfg.MCP.Timeout,
			})
		}
	} else {
		// Create MCP reverse proxies for each backend
		for _, backend := range s.cfg.MCP.Backends {
			// Set default timeout from global config if not set on backend
			if backend.Timeout == 0 {
				backend.Timeout = s.cfg.MCP.Timeout
			}
			s.setupMCPProxy(&backend)
		}
	}
	
	// Catch-all handler for undefined paths
	s.mux.HandleFunc("* /", func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debug("No backend configured for path", "path", r.URL.Path)
		http.Error(w, "Not found: no backend configured for this path", http.StatusNotFound)
	})
}

// setupMCPProxy creates and registers a reverse proxy for a specific MCP backend
func (s *Server) setupMCPProxy(backend *config.MCPBackend) {
	targetURL, err := url.Parse(backend.URL)
	if err != nil {
		s.logger.Error("Failed to parse target URL", 
			"name", backend.Name,
			"url", backend.URL, 
			"path", backend.Path,
			"error", err)
		return
	}

	// Create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	
	// Custom transport with timeout and instrumentation
	transport := &http.Transport{
		ResponseHeaderTimeout: backend.Timeout,
		// Add other transport configurations as needed
	}
	
	proxy.Transport = &instrumentedTransport{
		base:   transport,
		logger: s.logger.With("backend", backend.Name, "target", backend.URL),
	}

	// Setup director function
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		req2 := *req // Clone the request to make a copy
		originalDirector(&req2)
		
		// Handle path stripping if enabled
		if backend.StripPath && len(backend.Path) > 0 {
			// Remove the prefix from the path
			path := req.URL.Path
			if strings.HasPrefix(path, backend.Path) {
				// If the backend path is /api/v1 and the request is /api/v1/resource
				// this will change it to /resource
				req2.URL.Path = strings.TrimPrefix(path, backend.Path)
				// Ensure the path starts with "/"
				if !strings.HasPrefix(req2.URL.Path, "/") {
					req2.URL.Path = "/" + req2.URL.Path
				}
				s.logger.Debug("Stripped path prefix", 
					"original", path, 
					"new", req2.URL.Path,
					"backend", backend.Name)
			}
		}
		
		// Get claims from context
		if claims, ok := req.Context().Value(auth.ClaimsContextKey).(map[string]interface{}); ok {
			// Add useful claims as headers if needed
			if sub, ok := claims["sub"].(string); ok {
				req2.Header.Set("X-Subject", sub)
			}
			if email, ok := claims["email"].(string); ok {
				req2.Header.Set("X-Email", email)
			}
		}

		// Remove authentication header to prevent forwarding it
		req2.Header.Del("Authorization")
		
		// Add backend information
		req2.Header.Set("X-Proxy-Backend", backend.Name)
		
		*req = req2
	}

	// Setup error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		s.logger.Error("Proxy error", 
			"error", err, 
			"path", r.URL.Path, 
			"backend", backend.Name)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	}

	// Register proxy handler with auth middleware for this path
	s.logger.Info("Registering backend", 
		"name", backend.Name, 
		"path", backend.Path, 
		"target", backend.URL,
		"strip_prefix", backend.StripPath)
	
	// Create pattern based on the backend path
	pattern := "* " + backend.Path
	if !strings.HasSuffix(backend.Path, "/") {
		// Make sure both /api and /api/ match by adding a trailing /
		pattern += "/"
	}
	pattern += "*"
	
	s.mux.Handle(pattern, auth.AuthMiddleware(s.validator)(proxy))
}

// instrumentedTransport is an http.RoundTripper that logs requests and responses
type instrumentedTransport struct {
	base   http.RoundTripper
	logger *slog.Logger
}

// RoundTrip implements http.RoundTripper
func (t *instrumentedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Log the request
	t.logger.Debug("Proxy request", 
		"method", req.Method, 
		"path", req.URL.Path, 
		"remote_addr", req.RemoteAddr)

	// Send the request
	resp, err := t.base.RoundTrip(req)

	// Log the response
	if err != nil {
		t.logger.Error("Proxy error",
			"method", req.Method,
			"path", req.URL.Path,
			"error", err,
			"duration", time.Since(start))
		return nil, err
	}

	t.logger.Debug("Proxy response",
		"method", req.Method,
		"path", req.URL.Path,
		"status", resp.StatusCode,
		"duration", time.Since(start))

	return resp, nil
}