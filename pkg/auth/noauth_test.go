package auth

import (
	"context"
	"net/http"
	"testing"

	"github.com/ksysoev/smcp-proxy/pkg/test"
	"github.com/stretchr/testify/assert"
)

func TestNoAuthValidator(t *testing.T) {
	validator := NewNoAuthValidator()

	t.Run("ValidateToken returns empty claims with no error", func(t *testing.T) {
		// Test with various inputs, all should return empty claims and no error
		testCases := []struct {
			name  string
			token string
		}{
			{
				name:  "Empty token",
				token: "",
			},
			{
				name:  "Invalid token",
				token: "invalid-token",
			},
			{
				name:  "Valid looking token",
				token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				claims, err := validator.ValidateToken(context.Background(), tc.token)
				assert.NoError(t, err)
				assert.Empty(t, claims)
			})
		}
	})
}

func TestNoAuthMiddleware(t *testing.T) {
	// Create a next handler that records if it was called
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true

		// Check if empty claims were added to the context
		claims, ok := r.Context().Value(ClaimsContextKey).(map[string]interface{})
		assert.True(t, ok)
		assert.Empty(t, claims)
	})

	// Create the handler with middleware
	handler := NoAuthMiddleware(next)

	t.Run("Always passes through regardless of auth header", func(t *testing.T) {
		// Test cases with various authorization headers, all should pass through
		testCases := []struct {
			name           string
			authHeaderName string
			authHeaderVal  string
		}{
			{
				name:           "No auth header",
				authHeaderName: "",
				authHeaderVal:  "",
			},
			{
				name:           "Empty auth header",
				authHeaderName: "Authorization",
				authHeaderVal:  "",
			},
			{
				name:           "Invalid auth header",
				authHeaderName: "Authorization",
				authHeaderVal:  "InvalidFormat",
			},
			{
				name:           "Bearer token",
				authHeaderName: "Authorization",
				authHeaderVal:  "Bearer some-token",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Reset next called flag
				nextCalled = false

				// Create request with specified auth header
				req := test.NewTestRequest("GET", "/test", nil)
				if tc.authHeaderName != "" {
					req.Header.Set(tc.authHeaderName, tc.authHeaderVal)
				}
				res := test.NewTestResponse()

				// Call the handler
				handler.ServeHTTP(res, req)

				// Verify next was called and response is OK
				assert.True(t, nextCalled)
				assert.Equal(t, http.StatusOK, res.Code)
			})
		}
	})
}

func TestNoAuthTokenClient(t *testing.T) {
	client := NewNoAuthTokenClient()

	t.Run("GetToken returns empty token with no error", func(t *testing.T) {
		token, err := client.GetToken(context.Background())
		assert.NoError(t, err)
		assert.Empty(t, token)
	})
}

func TestNoAuthTransport(t *testing.T) {
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

	// Create the no-auth transport
	transport := &NoAuthTransport{
		Base:   baseTransport,
		Logger: test.NewTestLogger(),
	}

	t.Run("Passes request through without adding auth header", func(t *testing.T) {
		// Create a request
		req := test.NewTestRequest("GET", "https://example.com/test", nil)

		// Make a request using the transport
		resp, err := transport.RoundTrip(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify no Authorization header was added
		assert.NotNil(t, capturedRequest)
		assert.Empty(t, capturedRequest.Header.Get("Authorization"))
	})

	t.Run("Preserves existing auth header if present", func(t *testing.T) {
		// Create a request with an existing auth header
		req := test.NewTestRequest("GET", "https://example.com/test", nil)
		req.Header.Set("Authorization", "Bearer existing-token")

		// Make a request using the transport
		resp, err := transport.RoundTrip(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify the existing Authorization header was preserved
		assert.NotNil(t, capturedRequest)
		assert.Equal(t, "Bearer existing-token", capturedRequest.Header.Get("Authorization"))
	})
}