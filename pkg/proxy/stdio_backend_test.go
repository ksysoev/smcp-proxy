package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/auth"
	"github.com/ksysoev/smcp-proxy/pkg/config"
	"github.com/ksysoev/smcp-proxy/pkg/test"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock MCP process for testing
type mockMCPProcess struct {
	started      bool
	startErr     error
	stopErr      error
	requestInput map[string]interface{}
	requestErr   error
	responseData map[string]interface{}
}

func (m *mockMCPProcess) Start() error {
	m.started = true
	return m.startErr
}

func (m *mockMCPProcess) Stop() error {
	m.started = false
	return m.stopErr
}

func (m *mockMCPProcess) Request(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	m.requestInput = input
	return m.responseData, m.requestErr
}

func TestStdioBackendHandler(t *testing.T) {
	logger := test.NewTestLogger()

	t.Run("Create handler with valid config", func(t *testing.T) {
		backend := &config.MCPBackend{
			ID:        "test-stdio",
			Name:      "Test Stdio",
			Transport: config.StdioTransport,
			Path:      "/test",
			Model:     "claude-3",
			Stdio: config.StdioConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
		}

		// Create the handler
		handler, err := NewStdioBackendHandler(backend, logger)
		require.NoError(t, err)
		require.NotNil(t, handler)

		assert.Equal(t, backend, handler.backend)
		assert.NotNil(t, handler.process)
	})

	t.Run("Missing command", func(t *testing.T) {
		backend := &config.MCPBackend{
			ID:        "test-stdio-missing-command",
			Name:      "Test Stdio Missing Command",
			Transport: config.StdioTransport,
			Path:      "/test",
			Stdio:     config.StdioConfig{},
		}

		handler, err := NewStdioBackendHandler(backend, logger)
		require.Error(t, err)
		require.Nil(t, handler)

		// Verify error type
		assert.IsType(t, ErrInvalidBackendConfig(""), err)
	})

	t.Run("Start and stop handler", func(t *testing.T) {
		backend := &config.MCPBackend{
			ID:        "test-stdio-start-stop",
			Name:      "Test Stdio Start Stop",
			Transport: config.StdioTransport,
			Path:      "/test",
			Stdio: config.StdioConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
		}

		// Create a mock MCP process
		mockProcess := &mockMCPProcess{}

		// Create the handler with the mock process
		handler := &StdioBackendHandler{
			backend: backend,
			process: mockProcess,
			logger:  logger,
		}

		// Start the handler
		err := handler.Start()
		require.NoError(t, err)
		assert.True(t, mockProcess.started)

		// Stop the handler
		err = handler.Stop()
		require.NoError(t, err)
		assert.False(t, mockProcess.started)
	})

	t.Run("Start failure", func(t *testing.T) {
		backend := &config.MCPBackend{
			ID:        "test-stdio-start-fail",
			Name:      "Test Stdio Start Fail",
			Transport: config.StdioTransport,
			Path:      "/test",
			Stdio: config.StdioConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
		}

		// Create a mock MCP process that fails to start
		mockProcess := &mockMCPProcess{
			startErr: assert.AnError,
		}

		// Create the handler with the mock process
		handler := &StdioBackendHandler{
			backend: backend,
			process: mockProcess,
			logger:  logger,
		}

		// Start the handler
		err := handler.Start()
		require.Error(t, err)
		assert.False(t, mockProcess.started)
		assert.Equal(t, assert.AnError, err)
	})

	t.Run("Handle request", func(t *testing.T) {
		backend := &config.MCPBackend{
			ID:        "test-stdio-handle",
			Name:      "Test Stdio Handle",
			Transport: config.StdioTransport,
			Path:      "/prefix",
			StripPath: true,
			Model:     "claude-3",
			Stdio: config.StdioConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
		}

		// Create a mock MCP process
		mockProcess := &mockMCPProcess{
			responseData: map[string]interface{}{
				"response": "this is a test response",
				"status":   200.0,
				"headers": map[string]interface{}{
					"Content-Type": "application/json",
					"X-Test":       "test-value",
				},
			},
		}

		// Create the handler with the mock process
		handler := &StdioBackendHandler{
			backend: backend,
			process: mockProcess,
			logger:  logger,
		}

		// Start the handler
		err := handler.Start()
		require.NoError(t, err)

		// Create a request with JSON body
		requestBody := `{"prompt": "Hello, world!"}`
		req := test.NewTestRequest("POST", "/prefix/completions", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")

		// Add claims to context
		claims := jwt.MapClaims{
			"sub":   "user-123",
			"email": "user@example.com",
		}
		ctx := context.WithValue(req.Context(), auth.ClaimsContextKey, claims)
		req = req.WithContext(ctx)

		// Handle the request
		w := test.NewTestResponse()
		handler.Handle(w, req)

		// Verify response
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		assert.Equal(t, "test-value", w.Header().Get("X-Test"))

		// Parse response body
		var responseData map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &responseData)
		require.NoError(t, err)
		assert.Equal(t, "this is a test response", responseData["response"])

		// Verify request input to process
		require.NotNil(t, mockProcess.requestInput)
		assert.Equal(t, "/completions", mockProcess.requestInput["path"])
		assert.Equal(t, "POST", mockProcess.requestInput["method"])
		assert.Equal(t, "claude-3", mockProcess.requestInput["model"])

		// Verify headers were passed
		headers, ok := mockProcess.requestInput["headers"].(map[string]string)
		require.True(t, ok)
		assert.Equal(t, "application/json", headers["Content-Type"])

		// Verify user claims were passed
		user, ok := mockProcess.requestInput["user"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "user-123", user["sub"])
		assert.Equal(t, "user@example.com", user["email"])
	})

	t.Run("Handle request error", func(t *testing.T) {
		backend := &config.MCPBackend{
			ID:        "test-stdio-handle-error",
			Name:      "Test Stdio Handle Error",
			Transport: config.StdioTransport,
			Path:      "/test",
			Stdio: config.StdioConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
		}

		// Create a mock MCP process that returns an error
		mockProcess := &mockMCPProcess{
			requestErr: assert.AnError,
		}

		// Create the handler with the mock process
		handler := &StdioBackendHandler{
			backend: backend,
			process: mockProcess,
			logger:  logger,
		}

		// Start the handler
		err := handler.Start()
		require.NoError(t, err)

		// Create a request
		req := test.NewTestRequest("GET", "/test", nil)

		// Handle the request
		w := test.NewTestResponse()
		handler.Handle(w, req)

		// Verify response
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
