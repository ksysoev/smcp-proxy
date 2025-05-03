package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/config"
	"github.com/ksysoev/smcp-proxy/pkg/test"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockMCPBackendHandler is a mock implementation of MCPBackendHandler for testing
type MockMCPBackendHandler struct {
	startErr     error
	stopErr      error
	handleReq    *http.Request
	handleFunc   func(w http.ResponseWriter, r *http.Request)
	handleCalled bool
	startCalled  bool
	stopCalled   bool
}

func (m *MockMCPBackendHandler) Handle(w http.ResponseWriter, r *http.Request) {
	m.handleCalled = true
	m.handleReq = r
	if m.handleFunc != nil {
		m.handleFunc(w, r)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func (m *MockMCPBackendHandler) Start() error {
	m.startCalled = true
	return m.startErr
}

func (m *MockMCPBackendHandler) Stop() error {
	m.stopCalled = true
	return m.stopErr
}

// MockTokenValidator is a mock implementation of auth.TokenValidator for testing
type MockTokenValidator struct {
	validateErr    error
	validateClaims jwt.MapClaims
	validateToken  string
	validateCalled bool
}

func (m *MockTokenValidator) ValidateToken(ctx context.Context, token string) (jwt.MapClaims, error) {
	m.validateCalled = true
	m.validateToken = token
	return m.validateClaims, m.validateErr
}

// TestMockHandler will be accessed by the real implementation in mcp_backend.go

func TestServerSetupBackendHandler(t *testing.T) {
	logger := test.NewTestLogger()

	// Create a mock token validator
	mockValidator := &MockTokenValidator{
		validateClaims: jwt.MapClaims{"sub": "test-user"},
	}

	// Create server with mock validator and config
	server := &Server{
		logger:    logger,
		validator: mockValidator,
		mux:       http.NewServeMux(),
		backends:  make(map[string]MCPBackendHandler),
		cfg: &config.ServerConfig{
			Auth: struct {
				Mode config.AuthMode "mapstructure:\"mode\""
			}{
				Mode: config.NoAuthMode, // Default to no-auth mode for the test
			},
		},
	}

	// Create a test backend
	backend := &config.MCPBackend{
		ID:        "test-backend",
		Name:      "Test Backend",
		Transport: config.HTTPTransport,
		URL:       "http://example.com",
		Path:      "/api",
		StripPath: true,
	}

	t.Run("Setup backend handler", func(t *testing.T) {
		// Create a mock backend handler
		mockHandler := &MockMCPBackendHandler{}

		// Set the global mock handler for testing
		testMockHandler = mockHandler

		// Clean up after the test
		defer func() {
			testMockHandler = nil
		}()

		// Setup the backend handler by dereferencing the pointer
		err := server.setupBackendHandler(*backend)
		require.NoError(t, err)

		// Verify the mock handler was started
		assert.True(t, mockHandler.startCalled)

		// Verify the backend was added to the map
		assert.Contains(t, server.backends, "test-backend")
		assert.Equal(t, mockHandler, server.backends["test-backend"])

		// Basic test that the handler was registered
		// We skip the full request test since that's testing routing which is handled in another test
	})
}

func TestServerHealth(t *testing.T) {
	logger := test.NewTestLogger()

	// Create server
	server := &Server{
		logger:    logger,
		validator: &MockTokenValidator{},
		mux:       http.NewServeMux(),
		backends:  make(map[string]MCPBackendHandler),
		cfg: &config.ServerConfig{
			Metrics: struct {
				Path    string "mapstructure:\"path\""
				Enabled bool   "mapstructure:\"enabled\""
			}{
				Path:    "/metrics",
				Enabled: false,
			},
		},
	}

	// Initialize routes
	server.initRoutes()

	// Test health check endpoint
	req := test.NewTestRequest("GET", "/health", nil)
	w := test.NewTestResponse()

	server.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestServerModelsEndpoint(t *testing.T) {
	logger := test.NewTestLogger()

	// Create server with backends config
	server := &Server{
		logger:   logger,
		mux:      http.NewServeMux(),
		backends: make(map[string]MCPBackendHandler),
		cfg: &config.ServerConfig{
			MCP: struct {
				Backends  []config.MCPBackend "mapstructure:\"backends\""
				Endpoints []string            "mapstructure:\"endpoints\""
				Timeout   time.Duration       "mapstructure:\"timeout\""
			}{
				Backends: []config.MCPBackend{
					{
						ID:        "test-backend-1",
						Name:      "Test Backend 1",
						Model:     "claude-3-sonnet",
						MaxTokens: 200000,
						Path:      "/api/claude-3-sonnet",
					},
					{
						ID:        "test-backend-2",
						Name:      "Test Backend 2",
						Model:     "claude-3-opus",
						MaxTokens: 300000,
						Path:      "/api/claude-3-opus",
					},
				},
			},
		},
	}

	// Initialize routes
	server.initRoutes()

	// Test models endpoint
	req := test.NewTestRequest("GET", "/api/models", nil)
	w := test.NewTestResponse()

	server.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Parse the response
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify the response structure
	assert.Equal(t, "list", response["object"])
	data, ok := response["data"].([]interface{})
	require.True(t, ok)
	require.Len(t, data, 2)

	// Verify the first model
	model1, ok := data[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test-backend-1", model1["ID"])
	assert.Equal(t, "Test Backend 1", model1["Name"])
	assert.Equal(t, "claude-3-sonnet", model1["Model"])
	assert.Equal(t, float64(200000), model1["MaxTokens"])
	assert.Equal(t, "/api/claude-3-sonnet", model1["Path"])

	// Verify the second model
	model2, ok := data[1].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test-backend-2", model2["ID"])
	assert.Equal(t, "Test Backend 2", model2["Name"])
	assert.Equal(t, "claude-3-opus", model2["Model"])
	assert.Equal(t, float64(300000), model2["MaxTokens"])
	assert.Equal(t, "/api/claude-3-opus", model2["Path"])
}
