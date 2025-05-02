package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ksysoev/smcp-proxy/pkg/auth"
	"github.com/ksysoev/smcp-proxy/pkg/config"
)

// StdioBackendHandler handles requests to a stdio-based MCP backend
type StdioBackendHandler struct {
	backend *config.MCPBackend
	process *MCPProcess
	logger  *slog.Logger
}

// NewStdioBackendHandler creates a new stdio backend handler
func NewStdioBackendHandler(backend *config.MCPBackend, logger *slog.Logger) (*StdioBackendHandler, error) {
	// Validate backend configuration
	if backend.Stdio.Command == "" {
		return nil, ErrInvalidBackendConfig("command is required for stdio transport")
	}

	// Create MCP process manager
	process := NewMCPProcess(backend.ID, backend.Stdio, logger)

	return &StdioBackendHandler{
		backend: backend,
		process: process,
		logger:  logger.With("backend", backend.ID, "transport", "stdio"),
	}, nil
}

// Handle processes an HTTP request to the stdio MCP backend
func (h *StdioBackendHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Failed to read request body", "error", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse request body as JSON
	var inputJSON map[string]interface{}
	if err := json.Unmarshal(body, &inputJSON); err != nil {
		h.logger.Error("Failed to parse request body", "error", err)
		http.Error(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	// Get path for routing
	path := r.URL.Path
	if h.backend.StripPath && strings.HasPrefix(path, h.backend.Path) {
		path = strings.TrimPrefix(path, h.backend.Path)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}
	inputJSON["path"] = path

	// Add headers to input JSON
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	inputJSON["headers"] = headers

	// Add user information from claims
	if claims, ok := r.Context().Value(auth.ClaimsContextKey).(map[string]interface{}); ok {
		inputJSON["user"] = claims
	}

	// Set model if specified
	if h.backend.Model != "" {
		inputJSON["model"] = h.backend.Model
	}

	// Add method
	inputJSON["method"] = r.Method

	// Make the request to the MCP process
	response, err := h.process.Request(r.Context(), inputJSON)
	if err != nil {
		h.logger.Error("MCP process request failed", "error", err, "path", path)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set response headers if provided
	if responseHeaders, ok := response["headers"].(map[string]interface{}); ok {
		for key, value := range responseHeaders {
			if strValue, ok := value.(string); ok {
				w.Header().Set(key, strValue)
			}
		}
		delete(response, "headers")
	}

	// Set status code if provided
	status := http.StatusOK
	if statusCode, ok := response["status"].(float64); ok {
		status = int(statusCode)
		delete(response, "status")
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// Start initializes the stdio backend by starting the MCP process
func (h *StdioBackendHandler) Start() error {
	h.logger.Info("Starting stdio backend",
		"id", h.backend.ID,
		"name", h.backend.Name,
		"command", h.backend.Stdio.Command,
		"path", h.backend.Path)
	return h.process.Start()
}

// Stop gracefully shuts down the stdio backend
func (h *StdioBackendHandler) Stop() error {
	h.logger.Info("Stopping stdio backend", "id", h.backend.ID)
	return h.process.Stop()
}
