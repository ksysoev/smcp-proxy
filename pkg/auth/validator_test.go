package auth

import (
	"context"
	"net/http"
	"testing"

	"github.com/ksysoev/smcp-proxy/pkg/test"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware(t *testing.T) {
	// Create a mock validator
	mockValidator := &mockTokenValidator{}

	// Create the middleware
	middleware := AuthMiddleware(mockValidator)

	// Create a next handler that records if it was called
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true

		// Check if claims were added to the context
		claims, ok := r.Context().Value(ClaimsContextKey).(jwt.MapClaims)
		assert.True(t, ok)
		assert.Equal(t, "test-sub", claims["sub"])
	})

	// Create the handler with middleware
	handler := middleware(next)

	t.Run("Valid token passes through", func(t *testing.T) {
		// Set up mock validator to return valid claims
		mockValidator.claims = jwt.MapClaims{"sub": "test-sub"}
		mockValidator.err = nil

		// Create request with authorization header
		req := test.NewTestRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		res := test.NewTestResponse()

		// Call the handler
		handler.ServeHTTP(res, req)

		// Verify next was called and response is OK
		assert.True(t, nextCalled)
		assert.Equal(t, http.StatusOK, res.Code)
	})

	t.Run("Missing authorization header", func(t *testing.T) {
		// Reset next called flag
		nextCalled = false

		// Create request without authorization header
		req := test.NewTestRequest("GET", "/test", nil)
		res := test.NewTestResponse()

		// Call the handler
		handler.ServeHTTP(res, req)

		// Verify next was not called and response is Unauthorized
		assert.False(t, nextCalled)
		assert.Equal(t, http.StatusUnauthorized, res.Code)
	})

	t.Run("Invalid authorization header format", func(t *testing.T) {
		// Reset next called flag
		nextCalled = false

		// Create request with invalid authorization header
		req := test.NewTestRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		res := test.NewTestResponse()

		// Call the handler
		handler.ServeHTTP(res, req)

		// Verify next was not called and response is Unauthorized
		assert.False(t, nextCalled)
		assert.Equal(t, http.StatusUnauthorized, res.Code)
	})

	t.Run("Invalid token", func(t *testing.T) {
		// Reset next called flag
		nextCalled = false

		// Set up mock validator to return an error
		mockValidator.claims = nil
		mockValidator.err = ErrInvalidToken

		// Create request with authorization header
		req := test.NewTestRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		res := test.NewTestResponse()

		// Call the handler
		handler.ServeHTTP(res, req)

		// Verify next was not called and response is Unauthorized
		assert.False(t, nextCalled)
		assert.Equal(t, http.StatusUnauthorized, res.Code)
	})

	t.Run("Claims mismatch", func(t *testing.T) {
		// Reset next called flag
		nextCalled = false

		// Set up mock validator to return a claims mismatch error
		mockValidator.claims = nil
		mockValidator.err = ErrClaimsMismatch

		// Create request with authorization header
		req := test.NewTestRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer token-with-insufficient-claims")
		res := test.NewTestResponse()

		// Call the handler
		handler.ServeHTTP(res, req)

		// Verify next was not called and response is Forbidden
		assert.False(t, nextCalled)
		assert.Equal(t, http.StatusForbidden, res.Code)
	})
}

func TestClaimMatches(t *testing.T) {
	tests := []struct {
		name          string
		value         interface{}
		expectedValue string
		expected      bool
	}{
		{
			name:          "String exact match",
			value:         "admin",
			expectedValue: "admin",
			expected:      true,
		},
		{
			name:          "String no match",
			value:         "user",
			expectedValue: "admin",
			expected:      false,
		},
		{
			name:          "Array with match",
			value:         []interface{}{"user", "admin", "guest"},
			expectedValue: "admin",
			expected:      true,
		},
		{
			name:          "Array without match",
			value:         []interface{}{"user", "guest"},
			expectedValue: "admin",
			expected:      false,
		},
		{
			name:          "Number converted to string match",
			value:         42,
			expectedValue: "42",
			expected:      true,
		},
		{
			name:          "Boolean converted to string match",
			value:         true,
			expectedValue: "true",
			expected:      true,
		},
		{
			name:          "Boolean converted to string no match",
			value:         false,
			expectedValue: "true",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := claimMatches(tt.value, tt.expectedValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock token validator for testing
type mockTokenValidator struct {
	claims jwt.MapClaims
	err    error
}

func (m *mockTokenValidator) ValidateToken(ctx context.Context, token string) (jwt.MapClaims, error) {
	return m.claims, m.err
}
