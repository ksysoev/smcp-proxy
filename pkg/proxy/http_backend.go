package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ksysoev/smcp-proxy/pkg/auth"
	"github.com/ksysoev/smcp-proxy/pkg/config"
)

// HTTPBackendHandler handles requests to an HTTP-based MCP backend
type HTTPBackendHandler struct {
	backend *config.MCPBackend
	proxy   *httputil.ReverseProxy
	logger  *slog.Logger
}

// NewHTTPBackendHandler creates a new HTTP backend handler
func NewHTTPBackendHandler(backend config.MCPBackend, logger *slog.Logger) (*HTTPBackendHandler, error) {
	// Validate backend configuration
	if backend.URL == "" {
		return nil, ErrInvalidBackendConfig("URL is required for HTTP transport")
	}

	// Parse target URL
	targetURL, err := url.Parse(backend.URL)
	if err != nil {
		return nil, err
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Custom transport with timeout
	transport := &http.Transport{
		ResponseHeaderTimeout: backend.Timeout,
		// Add other transport configurations as needed
	}

	proxy.Transport = &instrumentedTransport{
		base:   transport,
		logger: logger.With("backend", backend.ID, "transport", "http"),
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
				logger.Debug("Stripped path prefix",
					"original", path,
					"new", req2.URL.Path,
					"backend", backend.ID)
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
		} else if claims, ok := req.Context().Value(auth.ClaimsContextKey).(jwt.MapClaims); ok {
			// Handle jwt.MapClaims type (used in tests)
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
		req2.Header.Set("X-Proxy-Backend", backend.ID)
		if backend.Model != "" {
			req2.Header.Set("X-Proxy-Model", backend.Model)
		}

		*req = req2
	}

	// Setup error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("Proxy error",
			"error", err,
			"path", r.URL.Path,
			"backend", backend.ID)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	}

	// Create the backend handler
	return &HTTPBackendHandler{
		backend: &backend,
		proxy:   proxy,
		logger:  logger.With("backend", backend.ID, "transport", "http"),
	}, nil
}

// Handle processes an HTTP request to the MCP backend
func (h *HTTPBackendHandler) Handle(w http.ResponseWriter, r *http.Request) {
	h.proxy.ServeHTTP(w, r)
}

// Start initializes the HTTP backend
func (h *HTTPBackendHandler) Start() error {
	h.logger.Info("Starting HTTP backend",
		"id", h.backend.ID,
		"name", h.backend.Name,
		"path", h.backend.Path)
	return nil
}

// Stop gracefully shuts down the HTTP backend
func (h *HTTPBackendHandler) Stop() error {
	h.logger.Info("Stopping HTTP backend", "id", h.backend.ID)
	return nil
}
