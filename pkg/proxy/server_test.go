package proxy

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/auth"
	"github.com/ksysoev/smcp-proxy/pkg/config"
	"github.com/ksysoev/smcp-proxy/pkg/test"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockMCPBackendHandler is a mock implementation of MCPBackendHandler for testing
type MockMCPBackendHandler struct {
	handleCalled bool
	startCalled  bool
	stopCalled   bool
	startErr     error
	stopErr      error
	handleReq    *http.Request
	handleFunc   func(w http.ResponseWriter, r *http.Request)
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
	validateCalled bool
	validateToken  string
	validateClaims jwt.MapClaims
	validateErr    error
}

func (m *MockTokenValidator) ValidateToken(ctx context.Context, token string) (jwt.MapClaims, error) {
	m.validateCalled = true
	m.validateToken = token
	return m.validateClaims, m.validateErr
}

func TestServerSetupBackendHandler(t *testing.T) {
	logger := test.NewTestLogger()

	// Create a mock token validator
	mockValidator := &MockTokenValidator{
		validateClaims: jwt.MapClaims{"sub": "test-user"},
	}

	// Create a basic server configuration
	cfg := &config.ServerConfig{}
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 8080

	// Create the server
	server := &Server{
		logger:    logger,
		validator: mockValidator,
		cfg:       cfg,
		mux:       http.NewServeMux(),
		backends:  make(map[string]MCPBackendHandler),
	}

	// Create a backend config
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

		// Set a factory function that returns our mock
		origNewMCPBackendHandler := NewMCPBackendHandler
		defer func() { NewMCPBackendHandler = origNewMCPBackendHandler }()

		NewMCPBackendHandler = func(b *config.MCPBackend, l *slog.Logger) (MCPBackendHandler, error) {
			return mockHandler, nil
		}

		// Setup the backend handler
		err := server.setupBackendHandler(backend)
		require.NoError(t, err)

		// Verify the mock handler was started
		assert.True(t, mockHandler.startCalled)

		// Verify the backend was added to the map
		assert.Contains(t, server.backends, "test-backend")
		assert.Equal(t, mockHandler, server.backends["test-backend"])

		// Test a request to the backend
		req := test.NewTestRequest("GET", "/api/resource", nil)
		w := test.NewTestResponse()

		// Add a mock token to the request
		req.Header.Set("Authorization", "Bearer test-token")

		// Dispatch the request
		server.mux.ServeHTTP(w, req)

		// Verify the validator was called
		assert.True(t, mockValidator.validateCalled)
		assert.Equal(t, "test-token", mockValidator.validateToken)

		// Verify the mock handler was called with the request
		assert.True(t, mockHandler.handleCalled)
		assert.Equal(t, "/api/resource", mockHandler.handleReq.URL.Path)
	})

	t.Run("Backend handler returns error", func(t *testing.T) {
		// Set a factory function that returns an error
		origNewMCPBackendHandler := NewMCPBackendHandler
		defer func() { NewMCPBackendHandler = origNewMCPBackendHandler }()

		NewMCPBackendHandler = func(b *config.MCPBackend, l *slog.Logger) (MCPBackendHandler, error) {
			return nil, assert.AnError
		}

		// Setup the backend handler
		err := server.setupBackendHandler(backend)
		require.Error(t, err)

		// Verify the backend was not added to the map
		assert.NotContains(t, server.backends, "test-backend-error")
	})

	t.Run("Backend handler start fails", func(t *testing.T) {
		// Create a mock backend handler that fails to start
		mockHandler := &MockMCPBackendHandler{
			startErr: assert.AnError,
		}

		// Set a factory function that returns our mock
		origNewMCPBackendHandler := NewMCPBackendHandler
		defer func() { NewMCPBackendHandler = origNewMCPBackendHandler }()

		NewMCPBackendHandler = func(b *config.MCPBackend, l *slog.Logger) (MCPBackendHandler, error) {
			return mockHandler, nil
		}

		// Setup the backend handler
		err := server.setupBackendHandler(backend)
		require.Error(t, err)

		// Verify the error type
		backendErr, ok := err.(ErrBackendStartFailed)
		require.True(t, ok)
		assert.Equal(t, "test-backend", backendErr.ID)
		assert.Equal(t, assert.AnError, backendErr.Cause)

		// Verify the backend was not added to the map
		assert.NotContains(t, server.backends, "test-backend-start-fail")
	})
}

func TestServerStop(t *testing.T) {
	logger := test.NewTestLogger()

	// Create a basic server configuration
	cfg := &config.ServerConfig{}
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 8080

	// Create a test HTTP server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Create the server
	server := &Server{
		logger:    logger,
		validator: &MockTokenValidator{},
		cfg:       cfg,
		mux:       http.NewServeMux(),
		backends:  make(map[string]MCPBackendHandler),
		httpServer: &http.Server{
			Addr:    "127.0.0.1:8080",
			Handler: http.NewServeMux(),
		},
	}

	// Add some mock backends
	mockBackend1 := &MockMCPBackendHandler{}
	mockBackend2 := &MockMCPBackendHandler{
		stopErr: assert.AnError, // This one will fail to stop
	}

	server.backends["backend1"] = mockBackend1
	server.backends["backend2"] = mockBackend2

	// Stop the server
	err := server.Stop(context.Background())
	require.NoError(t, err)

	// Verify both backends were stopped, even though one had an error
	assert.True(t, mockBackend1.stopCalled)
	assert.True(t, mockBackend2.stopCalled)
}

func TestModelsEndpoint(t *testing.T) {
	logger := test.NewTestLogger()

	// Create a mock token validator
	mockValidator := &MockTokenValidator{
		validateClaims: jwt.MapClaims{"sub": "test-user"},
	}

	// Create a server configuration with backends
	cfg := &config.ServerConfig{}
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 8080
	cfg.MCP.Backends = []*config.MCPBackend{
		{
			ID:        "backend1",
			Name:      "Model 1",
			Model:     "claude-3-opus",
			MaxTokens: 100000,
			Path:      "/v1/opus",
			Transport: config.HTTPTransport,
			URL:       "http://example.com/opus",
		},
		{
			ID:        "backend2",
			Name:      "Model 2",
			Model:     "claude-3-sonnet",
			MaxTokens: 200000,
			Path:      "/v1/sonnet",
			Transport: config.HTTPTransport,
			URL:       "http://example.com/sonnet",
		},
	}

	// Create the server
	server := &Server{
		logger:    logger,
		validator: mockValidator,
		cfg:       cfg,
		mux:       http.NewServeMux(),
		backends:  make(map[string]MCPBackendHandler),
	}

	// Initialize routes
	server.initRoutes()

	// Test the models endpoint
	req := test.NewTestRequest("GET", "/api/models", nil)
	w := test.NewTestResponse()

	// Dispatch the request
	server.mux.ServeHTTP(w, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Parse the response body
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
	assert.Equal(t, "backend1", model1["ID"])
	assert.Equal(t, "Model 1", model1["Name"])
	assert.Equal(t, "claude-3-opus", model1["Model"])
	assert.Equal(t, float64(100000), model1["MaxTokens"])
	assert.Equal(t, "/v1/opus", model1["Path"])

	// Verify the second model
	model2, ok := data[1].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "backend2", model2["ID"])
	assert.Equal(t, "Model 2", model2["Name"])
	assert.Equal(t, "claude-3-sonnet", model2["Model"])
	assert.Equal(t, float64(200000), model2["MaxTokens"])
	assert.Equal(t, "/v1/sonnet", model2["Path"])
}

func TestHealthEndpoint(t *testing.T) {
	logger := test.NewTestLogger()

	// Create a server configuration
	cfg := &config.ServerConfig{}
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 8080

	// Create the server
	server := &Server{
		logger: logger,
		cfg:    cfg,
		mux:    http.NewServeMux(),
	}

	// Initialize routes
	server.initRoutes()

	// Test the health endpoint
	req := test.NewTestRequest("GET", "/health", nil)
	w := test.NewTestResponse()

	// Dispatch the request
	server.mux.ServeHTTP(w, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}
