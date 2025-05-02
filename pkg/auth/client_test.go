package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ksysoev/smcp-proxy/pkg/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenTransport(t *testing.T) {
	// Create a mock token client
	mockClient := &mockTokenClient{
		token: "test-token",
		err:   nil,
	}

	// Create a base transport that captures the request
	var capturedRequest *http.Request
	baseTransport := &mockTransport{
		roundTripFn: func(req *http.Request) (*http.Response, error) {
			capturedRequest = req
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
			}, nil
		},
	}

	// Create the token transport
	transport := &TokenTransport{
		Base:   baseTransport,
		Client: mockClient,
		logger: test.NewTestLogger(),
	}

	t.Run("Adds token to request", func(t *testing.T) {
		// Create a request
		req := test.NewTestRequest("GET", "https://example.com/test", nil)

		// Make a request using the transport
		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify the token was added to the request
		require.NotNil(t, capturedRequest)
		assert.Equal(t, "Bearer test-token", capturedRequest.Header.Get("Authorization"))
	})

	t.Run("Caches errors when configured", func(t *testing.T) {
		// Set a previously successful token
		transport.LastToken = "previous-token"
		transport.CacheErrors = true

		// Set the mock client to return an error
		mockClient.err = assert.AnError
		mockClient.token = ""

		// Create a request
		req := test.NewTestRequest("GET", "https://example.com/test", nil)

		// Make a request using the transport
		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify the previous token was used
		require.NotNil(t, capturedRequest)
		assert.Equal(t, "Bearer previous-token", capturedRequest.Header.Get("Authorization"))
	})

	t.Run("Returns error when token acquisition fails and caching is disabled", func(t *testing.T) {
		// Disable caching
		transport.CacheErrors = false

		// Set the mock client to return an error
		mockClient.err = assert.AnError
		mockClient.token = ""

		// Create a request
		req := test.NewTestRequest("GET", "https://example.com/test", nil)

		// Make a request using the transport
		resp, err := transport.RoundTrip(req)
		require.Error(t, err)
		require.Nil(t, resp)
	})
}

// Mock token client for testing
type mockTokenClient struct {
	token string
	err   error
}

func (m *mockTokenClient) GetToken(ctx context.Context) (string, error) {
	return m.token, m.err
}

// Mock transport for testing
type mockTransport struct {
	roundTripFn func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFn(req)
}
