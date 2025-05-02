package proxy

import (
	"testing"

	"github.com/ksysoev/smcp-proxy/pkg/config"
	"github.com/ksysoev/smcp-proxy/pkg/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMCPBackendHandler(t *testing.T) {
	logger := test.NewTestLogger()

	t.Run("HTTP transport", func(t *testing.T) {
		backend := &config.MCPBackend{
			ID:        "test-http",
			Name:      "Test HTTP",
			Transport: config.HTTPTransport,
			URL:       "http://example.com",
			Path:      "/test",
		}

		handler, err := NewMCPBackendHandler(backend, logger)
		require.NoError(t, err)
		require.NotNil(t, handler)

		// Verify the handler type
		_, ok := handler.(*HTTPBackendHandler)
		assert.True(t, ok)
	})

	t.Run("Stdio transport", func(t *testing.T) {
		backend := &config.MCPBackend{
			ID:        "test-stdio",
			Name:      "Test Stdio",
			Transport: config.StdioTransport,
			Path:      "/test",
			Stdio: config.StdioConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
		}

		handler, err := NewMCPBackendHandler(backend, logger)
		require.NoError(t, err)
		require.NotNil(t, handler)

		// Verify the handler type
		_, ok := handler.(*StdioBackendHandler)
		assert.True(t, ok)
	})

	t.Run("Missing ID", func(t *testing.T) {
		backend := &config.MCPBackend{
			Name:      "Test Missing ID",
			Transport: config.HTTPTransport,
			URL:       "http://example.com",
			Path:      "/test",
		}

		handler, err := NewMCPBackendHandler(backend, logger)
		require.Error(t, err)
		require.Nil(t, handler)
	})

	t.Run("Invalid transport", func(t *testing.T) {
		backend := &config.MCPBackend{
			ID:        "test-invalid",
			Name:      "Test Invalid",
			Transport: "invalid",
			Path:      "/test",
		}

		handler, err := NewMCPBackendHandler(backend, logger)
		require.Error(t, err)
		require.Nil(t, handler)
	})

	t.Run("HTTP transport missing URL", func(t *testing.T) {
		backend := &config.MCPBackend{
			ID:        "test-http-missing-url",
			Name:      "Test HTTP Missing URL",
			Transport: config.HTTPTransport,
			Path:      "/test",
		}

		handler, err := NewMCPBackendHandler(backend, logger)
		require.Error(t, err)
		require.Nil(t, handler)

		// Check that we get an ErrInvalidBackendConfig
		_, ok := err.(ErrInvalidBackendConfig)
		assert.True(t, ok)
	})

	t.Run("Stdio transport missing command", func(t *testing.T) {
		backend := &config.MCPBackend{
			ID:        "test-stdio-missing-command",
			Name:      "Test Stdio Missing Command",
			Transport: config.StdioTransport,
			Path:      "/test",
			Stdio:     config.StdioConfig{
				// Command is missing
			},
		}

		handler, err := NewMCPBackendHandler(backend, logger)
		require.Error(t, err)
		require.Nil(t, handler)

		// Check that we get an ErrInvalidBackendConfig
		_, ok := err.(ErrInvalidBackendConfig)
		assert.True(t, ok)
	})
}

func TestListBackendModels(t *testing.T) {
	backends := []*config.MCPBackend{
		{
			ID:        "backend1",
			Name:      "Backend 1",
			Model:     "model1",
			MaxTokens: 1000,
			Path:      "/path1",
		},
		{
			ID:        "backend2",
			Name:      "Backend 2",
			Model:     "model2",
			MaxTokens: 2000,
			Path:      "/path2",
		},
	}

	models := ListBackendModels(backends)
	require.Len(t, models, 2)

	// Check first model
	assert.Equal(t, "backend1", models[0].ID)
	assert.Equal(t, "Backend 1", models[0].Name)
	assert.Equal(t, "model1", models[0].Model)
	assert.Equal(t, 1000, models[0].MaxTokens)
	assert.Equal(t, "/path1", models[0].Path)

	// Check second model
	assert.Equal(t, "backend2", models[1].ID)
	assert.Equal(t, "Backend 2", models[1].Name)
	assert.Equal(t, "model2", models[1].Model)
	assert.Equal(t, 2000, models[1].MaxTokens)
	assert.Equal(t, "/path2", models[1].Path)
}
