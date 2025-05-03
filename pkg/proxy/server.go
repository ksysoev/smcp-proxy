package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
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
	backends   map[string]MCPBackendHandler
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

	// Create the appropriate token validator based on auth mode
	var validator auth.TokenValidator
	if cfg.Auth.Mode == config.OIDCAuthMode {
		// Create OIDC token validator
		oidcValidator, err := auth.NewOIDCTokenValidator(
			ctx,
			cfg.OIDC.Issuers,
			cfg.OIDC.Audience,
			cfg.OIDC.RequiredClaims,
			cfg.OIDC.OptionalClaims,
			logger,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OIDC token validator: %w", err)
		}
		validator = oidcValidator
	} else {
		// Create no-op token validator for no-auth mode
		validator = auth.NewNoAuthValidator()
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
		backends:   make(map[string]MCPBackendHandler),
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

// Stop gracefully stops the proxy server and all backends
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping server")

	// Stop all backends
	for id, backend := range s.backends {
		if err := backend.Stop(); err != nil {
			s.logger.Error("Failed to stop backend", "id", id, "error", err)
		}
	}

	// Shutdown HTTP server
	return s.httpServer.Shutdown(ctx)
}

// initRoutes initializes the server routes
func (s *Server) initRoutes() {
	// Add health check endpoint
	s.mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Add metrics endpoint if enabled
	if s.cfg.Metrics.Enabled {
		s.mux.HandleFunc("GET "+s.cfg.Metrics.Path, func(w http.ResponseWriter, r *http.Request) {
			// Metrics implementation would go here
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Metrics would be here"))
		})
	}

	// Add models info endpoint
	s.mux.HandleFunc("GET /api/models", func(w http.ResponseWriter, r *http.Request) {
		models := ListBackendModels(s.cfg.MCP.Backends)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data":   models,
		}); err != nil {
			s.logger.Error("Failed to encode models response", "error", err)
		}
	})

	// Handle legacy endpoints configuration (for backward compatibility)
	if len(s.cfg.MCP.Endpoints) > 0 && len(s.cfg.MCP.Backends) == 0 {
		s.logger.Warn("Using deprecated 'endpoints' configuration. Please migrate to 'backends' configuration.")
		// Create a root backend for each endpoint
		for i, endpoint := range s.cfg.MCP.Endpoints {
			backend := &config.MCPBackend{
				ID:        fmt.Sprintf("legacy-%d", i),
				Name:      "Legacy Backend",
				Transport: config.HTTPTransport,
				URL:       endpoint,
				Path:      "/",
				StripPath: false,
				Timeout:   s.cfg.MCP.Timeout,
			}
			// Fix: Use the backend directly without taking its address
			// Create a copy of the backend struct to pass
			backendCopy := *backend
			if err := s.setupBackendHandler(backendCopy); err != nil {
				s.logger.Error("Failed to setup legacy backend", "url", endpoint, "error", err)
			}
		}
	} else {
		// Create handlers for each configured backend
		for _, backend := range s.cfg.MCP.Backends {
			// Set default timeout from global config if not set on backend
			if backend.Timeout == 0 {
				backend.Timeout = s.cfg.MCP.Timeout
			}

			// Generate an ID if not provided
			if backend.ID == "" {
				backend.ID = fmt.Sprintf("backend-%s", backend.Name)
			}

			// Set default transport if not provided
			if backend.Transport == "" {
				backend.Transport = config.HTTPTransport
			}

			// Fix: Use the backend directly without creating a pointer to a pointer
			if err := s.setupBackendHandler(backend); err != nil {
				s.logger.Error("Failed to setup backend",
					"id", backend.ID,
					"name", backend.Name,
					"path", backend.Path,
					"error", err)
			}
		}
	}

	// Catch-all handler for undefined paths
	s.mux.HandleFunc("* /", func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debug("No backend configured for path", "path", r.URL.Path)
		http.Error(w, "Not found: no backend configured for this path", http.StatusNotFound)
	})
}

// setupBackendHandler creates and registers a handler for a specific MCP backend
func (s *Server) setupBackendHandler(backend config.MCPBackend) error {
	// Create the appropriate backend handler based on transport
	handler, err := NewMCPBackendHandler(backend, s.logger)
	if err != nil {
		return fmt.Errorf("failed to create backend handler: %w", err)
	}

	// Start the backend
	if err := handler.Start(); err != nil {
		return ErrBackendStartFailed{ID: backend.ID, Cause: err}
	}

	// Store the handler for later reference
	s.backends[backend.ID] = handler

	// Register handler with auth middleware for this path
	s.logger.Info("Registering backend",
		"id", backend.ID,
		"name", backend.Name,
		"path", backend.Path,
		"transport", backend.Transport,
		"strip_path", backend.StripPath)

	// Create pattern based on the backend path
	pattern := "* " + backend.Path
	if !strings.HasSuffix(backend.Path, "/") {
		// Make sure both /api and /api/ match by adding a trailing /
		pattern += "/"
	}
	pattern += "*"

	// Create handler function
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		handler.Handle(w, r)
	}

	// Apply appropriate middleware based on auth mode
	if s.cfg.Auth.Mode == config.OIDCAuthMode {
		// Apply auth middleware for OIDC mode
		s.mux.Handle(pattern, auth.AuthMiddleware(s.validator)(http.HandlerFunc(handlerFunc)))
	} else {
		// Apply no-auth middleware for none mode
		s.mux.Handle(pattern, auth.NoAuthMiddleware(http.HandlerFunc(handlerFunc)))
	}

	return nil
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
