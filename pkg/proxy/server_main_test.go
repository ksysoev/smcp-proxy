package proxy

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/auth"
	"github.com/ksysoev/smcp-proxy/pkg/config"
	"github.com/ksysoev/smcp-proxy/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Create a variable that can be swapped during tests
var newOIDCTokenValidator = auth.NewOIDCTokenValidator

// mockMCPBackendHandlerFactory mocks the creation of backend handlers
type mockMCPBackendHandlerFactory struct {
	handlers map[string]*MockMCPBackendHandler
}

func (f *mockMCPBackendHandlerFactory) newHandler(backend config.MCPBackend, logger *slog.Logger) (MCPBackendHandler, error) {
	if backend.ID == "error-backend" {
		return nil, errors.New("forced error for testing")
	}

	handler := &MockMCPBackendHandler{}
	f.handlers[backend.ID] = handler
	return handler, nil
}

// TestNewServer tests server creation with various configurations
func TestNewServer(t *testing.T) {
	// Save original function and restore after test
	origNewMCPBackendHandler := NewMCPBackendHandler
	origNewTokenValidator := newOIDCTokenValidator

	defer func() {
		NewMCPBackendHandler = origNewMCPBackendHandler
		newOIDCTokenValidator = origNewTokenValidator
	}()

	// Mock the token validator
	newOIDCTokenValidator = func(ctx context.Context, issuers []string, audience string, requiredClaims, optionalClaims map[string]string, logger *slog.Logger) (auth.TokenValidator, error) {
		return &MockTokenValidator{}, nil
	}

	t.Run("Create server with valid config", func(t *testing.T) {
		// Create a mock handler factory
		factory := &mockMCPBackendHandlerFactory{
			handlers: make(map[string]*MockMCPBackendHandler),
		}

		// Replace the backend handler factory
		NewMCPBackendHandler = factory.newHandler

		// Create a basic config
		cfg := &config.ServerConfig{
			Server: struct {
				Host            string        `mapstructure:"host"`
				Port            int           `mapstructure:"port"`
				ReadTimeout     time.Duration `mapstructure:"read_timeout"`
				WriteTimeout    time.Duration `mapstructure:"write_timeout"`
				ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
			}{
				Host:            "127.0.0.1",
				Port:            8080,
				ReadTimeout:     30 * time.Second,
				WriteTimeout:    30 * time.Second,
				ShutdownTimeout: 10 * time.Second,
			},
			MCP: struct {
				Timeout   time.Duration       `mapstructure:"timeout"`
				Endpoints []string            `mapstructure:"endpoints"`
				Backends  []config.MCPBackend `mapstructure:"backends"`
			}{
				Timeout: 60 * time.Second,
				Backends: []config.MCPBackend{
					{
						ID:        "test-backend-1",
						Name:      "Test Backend 1",
						Transport: config.HTTPTransport,
						URL:       "http://example.com/backend1",
						Path:      "/api/backend1",
					},
					{
						ID:        "test-backend-2",
						Name:      "Test Backend 2",
						Transport: config.HTTPTransport,
						URL:       "http://example.com/backend2",
						Path:      "/api/backend2",
					},
				},
			},
			OIDC: struct {
				Issuers        []string          `mapstructure:"issuers"`
				Audience       string            `mapstructure:"audience"`
				RequiredClaims map[string]string `mapstructure:"required_claims"`
				OptionalClaims map[string]string `mapstructure:"optional_claims"`
			}{
				Issuers:  []string{"https://example.com"},
				Audience: "test-audience",
			},
			Metrics: struct {
				Path    string `mapstructure:"path"`
				Enabled bool   `mapstructure:"enabled"`
			}{
				Path:    "/metrics",
				Enabled: true,
			},
			TLS: struct {
				CertFile string `mapstructure:"cert_file"`
				KeyFile  string `mapstructure:"key_file"`
				Enabled  bool   `mapstructure:"enabled"`
			}{
				Enabled: false,
			},
		}

		// Create the server
		ctx := context.Background()
		server, err := NewServer(ctx, cfg, test.NewTestLogger())
		require.NoError(t, err)
		require.NotNil(t, server)

		// Verify basics
		assert.Equal(t, "127.0.0.1:8080", server.httpServer.Addr)
		assert.Equal(t, 30*time.Second, server.httpServer.ReadTimeout)
		assert.Equal(t, 30*time.Second, server.httpServer.WriteTimeout)
		assert.NotNil(t, server.mux)
		assert.NotNil(t, server.validator)
		assert.NotNil(t, server.backends)
		assert.Len(t, server.backends, 2)

		// Verify each backend was created and started
		for _, backendID := range []string{"test-backend-1", "test-backend-2"} {
			assert.Contains(t, server.backends, backendID)

			// Confirm it was created via the factory
			mockHandler, exists := factory.handlers[backendID]
			require.True(t, exists)
			assert.True(t, mockHandler.startCalled)
		}
	})

	t.Run("Error creating server - token validator fails", func(t *testing.T) {
		// Mock the token validator to return an error
		newOIDCTokenValidator = func(ctx context.Context, issuers []string, audience string, requiredClaims, optionalClaims map[string]string, logger *slog.Logger) (auth.TokenValidator, error) {
			return nil, errors.New("validator creation failed")
		}

		// Create a basic config
		cfg := &config.ServerConfig{
			OIDC: struct {
				Issuers        []string          `mapstructure:"issuers"`
				Audience       string            `mapstructure:"audience"`
				RequiredClaims map[string]string `mapstructure:"required_claims"`
				OptionalClaims map[string]string `mapstructure:"optional_claims"`
			}{
				Issuers:  []string{"https://example.com"},
				Audience: "test-audience",
			},
		}

		// Create the server
		ctx := context.Background()
		server, err := NewServer(ctx, cfg, test.NewTestLogger())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create token validator")
		assert.Nil(t, server)
	})

	t.Run("Handle backend setup errors", func(t *testing.T) {
		// Mock token validator back to success
		newOIDCTokenValidator = func(ctx context.Context, issuers []string, audience string, requiredClaims, optionalClaims map[string]string, logger *slog.Logger) (auth.TokenValidator, error) {
			return &MockTokenValidator{}, nil
		}

		// Create a mock handler factory
		factory := &mockMCPBackendHandlerFactory{
			handlers: make(map[string]*MockMCPBackendHandler),
		}

		// Replace the backend handler factory
		NewMCPBackendHandler = factory.newHandler

		// Create a config with a backend that will fail to create
		cfg := &config.ServerConfig{
			Server: struct {
				Host            string        `mapstructure:"host"`
				Port            int           `mapstructure:"port"`
				ReadTimeout     time.Duration `mapstructure:"read_timeout"`
				WriteTimeout    time.Duration `mapstructure:"write_timeout"`
				ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
			}{
				Host:            "127.0.0.1",
				Port:            8080,
				ReadTimeout:     30 * time.Second,
				WriteTimeout:    30 * time.Second,
				ShutdownTimeout: 10 * time.Second,
			},
			MCP: struct {
				Timeout   time.Duration       `mapstructure:"timeout"`
				Endpoints []string            `mapstructure:"endpoints"`
				Backends  []config.MCPBackend `mapstructure:"backends"`
			}{
				Timeout: 60 * time.Second,
				Backends: []config.MCPBackend{
					{
						ID:        "test-backend",
						Name:      "Test Backend",
						Transport: config.HTTPTransport,
						URL:       "http://example.com/test",
						Path:      "/api/test",
					},
					{
						ID:        "error-backend", // This ID triggers an error in our mock factory
						Name:      "Error Backend",
						Transport: config.HTTPTransport,
						URL:       "http://example.com/error",
						Path:      "/api/error",
					},
				},
			},
			OIDC: struct {
				Issuers        []string          `mapstructure:"issuers"`
				Audience       string            `mapstructure:"audience"`
				RequiredClaims map[string]string `mapstructure:"required_claims"`
				OptionalClaims map[string]string `mapstructure:"optional_claims"`
			}{
				Issuers:  []string{"https://example.com"},
				Audience: "test-audience",
			},
		}

		// Create the server
		ctx := context.Background()
		server, err := NewServer(ctx, cfg, test.NewTestLogger())

		// Server should be created, but we should have logged error for the error-backend
		require.NoError(t, err) // Overall server creation shouldn't fail just because one backend fails
		require.NotNil(t, server)

		// Verify we only have the successful backend
		assert.Len(t, server.backends, 1)
		assert.Contains(t, server.backends, "test-backend")
		assert.NotContains(t, server.backends, "error-backend")
	})

	t.Run("Legacy endpoints configuration", func(t *testing.T) {
		// Create a mock handler factory
		factory := &mockMCPBackendHandlerFactory{
			handlers: make(map[string]*MockMCPBackendHandler),
		}

		// Replace the backend handler factory
		NewMCPBackendHandler = factory.newHandler

		// Create a config with legacy endpoints instead of backends
		cfg := &config.ServerConfig{
			Server: struct {
				Host            string        `mapstructure:"host"`
				Port            int           `mapstructure:"port"`
				ReadTimeout     time.Duration `mapstructure:"read_timeout"`
				WriteTimeout    time.Duration `mapstructure:"write_timeout"`
				ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
			}{
				Host:            "127.0.0.1",
				Port:            8080,
				ReadTimeout:     30 * time.Second,
				WriteTimeout:    30 * time.Second,
				ShutdownTimeout: 10 * time.Second,
			},
			MCP: struct {
				Timeout   time.Duration       `mapstructure:"timeout"`
				Endpoints []string            `mapstructure:"endpoints"`
				Backends  []config.MCPBackend `mapstructure:"backends"`
			}{
				Timeout:   60 * time.Second,
				Endpoints: []string{"http://legacy1.example.com", "http://legacy2.example.com"},
				Backends:  []config.MCPBackend{}, // Empty to trigger legacy mode
			},
			OIDC: struct {
				Issuers        []string          `mapstructure:"issuers"`
				Audience       string            `mapstructure:"audience"`
				RequiredClaims map[string]string `mapstructure:"required_claims"`
				OptionalClaims map[string]string `mapstructure:"optional_claims"`
			}{
				Issuers:  []string{"https://example.com"},
				Audience: "test-audience",
			},
		}

		// Create the server
		ctx := context.Background()
		server, err := NewServer(ctx, cfg, test.NewTestLogger())

		require.NoError(t, err)
		require.NotNil(t, server)

		// Verify we have two backends created from legacy endpoints
		assert.Len(t, server.backends, 2)

		// Legacy backends have IDs like "legacy-0", "legacy-1", etc.
		assert.Contains(t, factory.handlers, "legacy-0")
		assert.Contains(t, factory.handlers, "legacy-1")
	})
}

// TestServerStartStop tests the server start and stop functionality
func TestServerStartStop(t *testing.T) {
	// Mock HTTP server
	httpServer := &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: http.NewServeMux(),
	}

	// Create a server with mock backends
	server := &Server{
		httpServer: httpServer,
		logger:     test.NewTestLogger(),
		backends: map[string]MCPBackendHandler{
			"backend1": &MockMCPBackendHandler{},
			"backend2": &MockMCPBackendHandler{},
		},
		cfg: &config.ServerConfig{
			TLS: struct {
				CertFile string `mapstructure:"cert_file"`
				KeyFile  string `mapstructure:"key_file"`
				Enabled  bool   `mapstructure:"enabled"`
			}{
				Enabled: false,
			},
		},
	}

	// Create a test HTTP server that can be shut down
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Replace the ListenAndServe function to avoid actually starting a server
	origListenAndServe := httpServer.ListenAndServe
	httpServer.ListenAndServe = func() error {
		return http.ErrServerClosed // Simulate that the server is closed
	}
	defer func() {
		httpServer.ListenAndServe = origListenAndServe
	}()

	// Test we can call Start without error
	err := server.Start()
	assert.Equal(t, http.ErrServerClosed, err) // It returns the simulated error

	// Test the Stop method
	ctx := context.Background()
	err = server.Stop(ctx)
	assert.NoError(t, err)

	// Verify all backends were stopped
	for id, backend := range server.backends {
		mockBackend, ok := backend.(*MockMCPBackendHandler)
		if ok {
			assert.True(t, mockBackend.stopCalled, "Backend %s was not stopped", id)
		}
	}
}

// TestInstrumentedTransport tests the instrumented transport wrapper
func TestInstrumentedTransport(t *testing.T) {
	// Create a mock base transport
	baseTransport := &mockTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
			}, nil
		},
	}

	// Create an instrumented transport
	transport := &instrumentedTransport{
		base:   baseTransport,
		logger: test.NewTestLogger(),
	}

	// Create a request
	req, err := http.NewRequest("GET", "https://example.com/test", nil)
	require.NoError(t, err)

	// Perform a round trip
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test with an error from the base transport
	baseTransport.roundTripFunc = func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("transport error")
	}

	resp, err = transport.RoundTrip(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transport error")
	assert.Nil(t, resp)
}

// mockTransport implements http.RoundTripper for testing
type mockTransport struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}
