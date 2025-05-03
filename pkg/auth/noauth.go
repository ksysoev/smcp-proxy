package auth

import (
	"context"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

// NoAuthValidator is a no-op implementation of TokenValidator that doesn't perform any validation
type NoAuthValidator struct{}

// NewNoAuthValidator creates a new NoAuthValidator
func NewNoAuthValidator() *NoAuthValidator {
	return &NoAuthValidator{}
}

// ValidateToken always returns an empty claims map and no error
func (v *NoAuthValidator) ValidateToken(_ context.Context, _ string) (jwt.MapClaims, error) {
	// Return empty claims map
	return jwt.MapClaims{}, nil
}

// NoAuthMiddleware is a middleware that skips authentication
func NoAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add empty claims to the request context
		ctx := context.WithValue(r.Context(), ClaimsContextKey, jwt.MapClaims{})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}