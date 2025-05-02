package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrMissingToken is returned when the token is missing
	ErrMissingToken = errors.New("missing token")
	// ErrInvalidToken is returned when the token is invalid
	ErrInvalidToken = errors.New("invalid token")
	// ErrClaimsMismatch is returned when the claims do not match
	ErrClaimsMismatch = errors.New("claims mismatch")
)

// TokenValidator is responsible for validating OIDC tokens
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (jwt.MapClaims, error)
}

// OIDCTokenValidator implements TokenValidator using OIDC providers
type OIDCTokenValidator struct {
	requiredClaims map[string]string
	optionalClaims map[string]string
	providers      map[string]*oidc.Provider
	verifiers      map[string]*oidc.IDTokenVerifier
	logger         *slog.Logger
	audience       string
	issuers        []string
	mu             sync.RWMutex
}

// NewOIDCTokenValidator creates a new OIDC token validator
func NewOIDCTokenValidator(
	ctx context.Context,
	issuers []string,
	audience string,
	requiredClaims map[string]string,
	optionalClaims map[string]string,
	logger *slog.Logger,
) (*OIDCTokenValidator, error) {
	if logger == nil {
		logger = slog.Default()
	}

	validator := &OIDCTokenValidator{
		issuers:        issuers,
		audience:       audience,
		requiredClaims: requiredClaims,
		optionalClaims: optionalClaims,
		providers:      make(map[string]*oidc.Provider),
		verifiers:      make(map[string]*oidc.IDTokenVerifier),
		logger:         logger,
	}

	// Initialize providers and verifiers
	for _, issuerURL := range issuers {
		if err := validator.initProvider(ctx, issuerURL); err != nil {
			logger.Warn("Failed to initialize provider", "issuer", issuerURL, "error", err)
			// Continue with other issuers, we'll try to initialize this one later
			continue
		}
	}

	if len(validator.providers) == 0 {
		return nil, fmt.Errorf("failed to initialize any OIDC providers")
	}

	return validator, nil
}

// initProvider initializes an OIDC provider and verifier
func (v *OIDCTokenValidator) initProvider(ctx context.Context, issuerURL string) error {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	config := &oidc.Config{
		ClientID: v.audience,
	}

	verifier := provider.Verifier(config)

	v.mu.Lock()
	defer v.mu.Unlock()

	v.providers[issuerURL] = provider
	v.verifiers[issuerURL] = verifier

	return nil
}

// ValidateToken validates the OIDC token
func (v *OIDCTokenValidator) ValidateToken(ctx context.Context, tokenStr string) (jwt.MapClaims, error) {
	if tokenStr == "" {
		return nil, ErrMissingToken
	}

	// Parse the token without validation first to determine the issuer
	token, _, err := new(jwt.Parser).ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("%w: cannot parse claims", ErrInvalidToken)
	}

	// Get the issuer from the claims
	issuer, ok := claims["iss"].(string)
	if !ok {
		return nil, fmt.Errorf("%w: missing issuer claim", ErrInvalidToken)
	}

	// Check if the issuer is trusted
	trusted := false
	for _, trustedIssuer := range v.issuers {
		if issuer == trustedIssuer {
			trusted = true
			break
		}
	}

	if !trusted {
		return nil, fmt.Errorf("%w: untrusted issuer: %s", ErrInvalidToken, issuer)
	}

	// Get the verifier for this issuer
	v.mu.RLock()
	verifier, ok := v.verifiers[issuer]
	v.mu.RUnlock()

	if !ok {
		// Try to initialize the provider if not available
		if err := v.initProvider(ctx, issuer); err != nil {
			return nil, fmt.Errorf("%w: issuer not configured: %s", ErrInvalidToken, issuer)
		}

		v.mu.RLock()
		verifier = v.verifiers[issuer]
		v.mu.RUnlock()
	}

	// Verify the token
	idToken, err := verifier.Verify(ctx, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	// Extract claims from the verified token
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("%w: failed to extract claims: %v", ErrInvalidToken, err)
	}

	// Verify required claims
	if err := v.verifyClaims(claims, v.requiredClaims, true); err != nil {
		return nil, err
	}

	// Verify optional claims if present
	if err := v.verifyClaims(claims, v.optionalClaims, false); err != nil {
		return nil, err
	}

	return claims, nil
}

// verifyClaims verifies that the claims match the expected values
func (v *OIDCTokenValidator) verifyClaims(claims jwt.MapClaims, expectedClaims map[string]string, required bool) error {
	for claim, expectedValue := range expectedClaims {
		value, exists := claims[claim]
		if !exists {
			if required {
				return fmt.Errorf("%w: missing required claim: %s", ErrClaimsMismatch, claim)
			}
			continue
		}

		if !claimMatches(value, expectedValue) {
			return fmt.Errorf("%w: claim %s has value %v, expected %s", ErrClaimsMismatch, claim, value, expectedValue)
		}
	}

	return nil
}

// claimMatches checks if a claim matches the expected value
// It supports string values and string arrays (checking if the expected value is in the array)
func claimMatches(value interface{}, expectedValue string) bool {
	switch v := value.(type) {
	case string:
		return v == expectedValue
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok && str == expectedValue {
				return true
			}
		}
		return false
	default:
		return fmt.Sprintf("%v", value) == expectedValue
	}
}

// AuthMiddleware is a middleware that validates OIDC tokens
func AuthMiddleware(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Extract the token from the Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Unauthorized: missing authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, "Unauthorized: invalid authorization header", http.StatusUnauthorized)
				return
			}

			token := parts[1]

			// Validate the token
			claims, err := validator.ValidateToken(ctx, token)
			if err != nil {
				var statusCode int
				var message string

				switch {
				case errors.Is(err, ErrMissingToken):
					statusCode = http.StatusUnauthorized
					message = "Unauthorized: missing token"
				case errors.Is(err, ErrInvalidToken):
					statusCode = http.StatusUnauthorized
					message = "Unauthorized: invalid token"
				case errors.Is(err, ErrClaimsMismatch):
					statusCode = http.StatusForbidden
					message = "Forbidden: insufficient permissions"
				default:
					statusCode = http.StatusInternalServerError
					message = "Internal server error"
				}

				http.Error(w, message, statusCode)
				return
			}

			// Add claims to the request context
			ctx = context.WithValue(ctx, ClaimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsContextKey is the key for storing claims in the request context
type claimsContextKey struct{}

// ClaimsContextKey is the context key for the claims
var ClaimsContextKey = claimsContextKey{}
