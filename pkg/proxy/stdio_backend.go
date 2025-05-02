package proxy

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ksysoev/smcp-proxy/pkg/auth"
	"github.com/ksysoev/smcp-proxy/pkg/config"
)

// MCPProcessInterface defines the interface for interacting with an MCP process
type MCPProcessInterface interface {
	Start() error
	Stop() error
	Request(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

// StdioBackendHandler handles requests to a stdio-based MCP backend
type StdioBackendHandler struct {
	backend *config.MCPBackend
	process MCPProcessInterface
	logger  *slog.Logger
}

// NewStdioBackendHandler creates a new stdio backend handler
func NewStdioBackendHandler(backend config.MCPBackend, logger *slog.Logger) (*StdioBackendHandler, error) {
	// Validate backend configuration
	if backend.Stdio.Command == "" {
		return nil, ErrInvalidBackendConfig("command is required for stdio transport")
	}

	// Create MCP process manager
	process := NewMCPProcess(backend.ID, backend.Stdio, logger)

	// Create a copy of the backend to store as a pointer
	backendCopy := backend
	return &StdioBackendHandler{
		backend: &backendCopy,
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
		h.logger.Error("Failed to parse request body as JSON", "error", err)
		http.Error(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	// Add model if not specified
	if _, ok := inputJSON["model"]; !ok && h.backend.Model != "" {
		inputJSON["model"] = h.backend.Model
	}

	// Add user info from auth context if available
	if claims, ok := r.Context().Value(auth.ClaimsContextKey).(jwt.MapClaims); ok {
		if sub, ok := claims["sub"].(string); ok {
			inputJSON["user"] = sub
		}
	}

	// Process the request
	h.logger.Debug("Sending request to MCP process", "request", inputJSON)
	responseJSON, err := h.process.Request(r.Context(), inputJSON)
	if err != nil {
		h.logger.Error("Failed to process request", "error", err)
		http.Error(w, "Failed to process request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responseJSON)
}

// Start initializes the backend
func (h *StdioBackendHandler) Start() error {
	h.logger.Info("Starting stdio backend", "command", h.backend.Stdio.Command)
	return h.process.Start()
}

// Stop gracefully shuts down the backend
func (h *StdioBackendHandler) Stop() error {
	h.logger.Info("Stopping stdio backend")
	return h.process.Stop()
}
