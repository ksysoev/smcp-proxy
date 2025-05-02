package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ksysoev/smcp-proxy/pkg/config"
)

// MCPBackendHandler defines the interface for handling MCP backend requests
type MCPBackendHandler interface {
	// Handle processes a request to the MCP backend
	Handle(w http.ResponseWriter, r *http.Request)
	// Start initializes the backend
	Start() error
	// Stop gracefully shuts down the backend
	Stop() error
}

// NewMCPBackendHandler creates a new MCP backend handler based on the transport type
func NewMCPBackendHandler(backend *config.MCPBackend, logger *slog.Logger) (MCPBackendHandler, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Ensure backend has an ID
	if backend.ID == "" {
		return nil, fmt.Errorf("backend must have an ID")
	}

	// Create handler based on transport type
	switch backend.Transport {
	case config.HTTPTransport:
		return NewHTTPBackendHandler(backend, logger)
	case config.StdioTransport:
		return NewStdioBackendHandler(backend, logger)
	default:
		return nil, fmt.Errorf("unsupported transport type: %s", backend.Transport)
	}
}

// BackendModelInfo contains information about the MCP backend model
type BackendModelInfo struct {
	// ID is a unique identifier for the backend
	ID string
	// Name is a human-readable name for the backend
	Name string
	// Model is the Anthropic model associated with this backend
	Model string
	// MaxTokens is the maximum number of tokens for this model
	MaxTokens int
	// Path is the URL path prefix for this backend
	Path string
}

// ListBackendModels returns a list of available backend models
func ListBackendModels(backends []*config.MCPBackend) []BackendModelInfo {
	models := make([]BackendModelInfo, 0, len(backends))

	for _, backend := range backends {
		models = append(models, BackendModelInfo{
			ID:        backend.ID,
			Name:      backend.Name,
			Model:     backend.Model,
			MaxTokens: backend.MaxTokens,
			Path:      backend.Path,
		})
	}

	return models
}
