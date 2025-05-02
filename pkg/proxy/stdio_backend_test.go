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

// Using MCPProcessInterface already defined in stdio_backend.go

// Mock MCP process for testing
type mockMCPProcess struct {
	startErr     error
	stopErr      error
	requestErr   error
	requestInput map[string]interface{}
	responseData map[string]interface{}
	started      bool
}

func (m *mockMCPProcess) Start() error {
	m.started = true
	return m.startErr
}

func (m *mockMCPProcess) Stop() error {
	return m.stopErr
}

func (m *mockMCPProcess) Request(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	m.requestInput = input
	if m.responseData == nil {
		m.responseData = map[string]interface{}{
			"id":      "test-response",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "claude-3-test",
		}
	}
	return m.responseData, m.requestErr
}

func TestStdioBackendHandler(t *testing.T) {
	logger := test.NewTestLogger()

	t.Run("Handler initialization", func(t *testing.T) {
		// Create a valid backend configuration
		backend := &config.MCPBackend{
			ID:        "test-stdio",
			Name:      "Test Stdio Backend",
			Transport: config.StdioTransport,
			Path:      "/api/test",
			Stdio: config.StdioConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
		}

		// Create a new handler
		handler, err := NewStdioBackendHandler(*backend, logger)
		require.NoError(t, err)
		assert.NotNil(t, handler)
		assert.Equal(t, backend, handler.backend)
		assert.NotNil(t, handler.process)
		assert.NotNil(t, handler.logger)
	})

	t.Run("Start and stop handler", func(t *testing.T) {
		// Create a valid backend configuration
		backend := &config.MCPBackend{
			ID:        "test-stdio",
			Name:      "Test Stdio Backend",
			Transport: config.StdioTransport,
			Path:      "/api/test",
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
	})

	t.Run("Handle request", func(t *testing.T) {
		// Create a valid backend configuration
		backend := &config.MCPBackend{
			ID:        "test-stdio",
			Name:      "Test Stdio Backend",
			Transport: config.StdioTransport,
			Path:      "/api/test",
			Model:     "claude-3-test",
			Stdio: config.StdioConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
		}

		// Create a mock MCP process
		mockProcess := &mockMCPProcess{
			responseData: map[string]interface{}{
				"id":      "test-response",
				"object":  "chat.completion",
				"created": time.Now().Unix(),
				"model":   "claude-3-test",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "This is a test response",
						},
					},
				},
			},
		}

		// Create the handler with the mock process
		handler := &StdioBackendHandler{
			backend: backend,
			process: mockProcess,
			logger:  logger,
		}

		// Create a test request
		testRequestBody := map[string]interface{}{
			"model": "claude-3-test",
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Hello, Claude!",
				},
			},
		}
		reqBody, err := json.Marshal(testRequestBody)
		require.NoError(t, err)

		req := test.NewTestRequest("POST", "/api/test", strings.NewReader(string(reqBody)))
		req.Header.Set("Content-Type", "application/json")
		// Add auth info
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsContextKey, jwt.MapClaims{
			"sub": "test-user",
		}))

		// Create a test response writer
		w := test.NewTestResponse()

		// Handle the request
		handler.Handle(w, req)

		// Verify the response
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		// Verify the process was called with the right input
		require.NotNil(t, mockProcess.requestInput)

		// Parse the response
		var responseBody map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &responseBody)
		require.NoError(t, err)

		assert.Equal(t, "test-response", responseBody["id"])
		assert.Equal(t, "chat.completion", responseBody["object"])
		assert.Equal(t, "claude-3-test", responseBody["model"])

		choices, ok := responseBody["choices"].([]interface{})
		require.True(t, ok)
		require.Len(t, choices, 1)

		choice, ok := choices[0].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(0), choice["index"])

		message, ok := choice["message"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "assistant", message["role"])
		assert.Equal(t, "This is a test response", message["content"])
	})
}
