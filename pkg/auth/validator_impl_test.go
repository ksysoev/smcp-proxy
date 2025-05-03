package auth

import (
	"context"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTokenValidator is a test implementation of TokenValidator
type mockTokenValidator struct {
	requiredClaims map[string]string
	optionalClaims map[string]string
}

// ValidateToken implements the TokenValidator interface for testing
func (m *mockTokenValidator) ValidateToken(ctx context.Context, tokenStr string) (jwt.MapClaims, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	// Validate required claims
	if err := m.verifyClaims(claims, m.requiredClaims, true); err != nil {
		return nil, err
	}

	// Validate optional claims
	if err := m.verifyClaims(claims, m.optionalClaims, false); err != nil {
		return nil, err
	}

	return claims, nil
}

// verifyClaims verifies that the claims match the expected values
func (m *mockTokenValidator) verifyClaims(claims jwt.MapClaims, expectedClaims map[string]string, required bool) error {
	for claim, expectedValue := range expectedClaims {
		value, exists := claims[claim]
		if !exists {
			if required {
				return ErrClaimsMismatch
			}
			continue
		}

		if !claimMatches(value, expectedValue) {
			return ErrClaimsMismatch
		}
	}
	return nil
}

// TestOIDCTokenValidator_ValidateToken tests the token validation functionality
func TestOIDCTokenValidator_ValidateToken(t *testing.T) {
	// Skip if we can't create real validators due to external dependencies
	t.Skip("This test requires network connectivity to OIDC providers, skipping")

	t.Run("Validate claims requirements", func(t *testing.T) {
		// Create a validator with claim requirements
		ctx := context.Background()
		validator, err := NewOIDCTokenValidator(
			ctx,
			[]string{"https://example.com"},
			"test-audience",
			map[string]string{
				"role": "admin",
			},
			map[string]string{
				"department": "engineering",
			},
			nil,
		)
		require.NoError(t, err)

		// Create a JWT with matching claims
		token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
			"iss":        "https://example.com",
			"sub":        "test-subject",
			"role":       "admin",
			"department": "engineering",
		})
		tokenStr, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
		require.NoError(t, err)

		// Create mock validator for testing
		mockValidator := &mockTokenValidator{
			requiredClaims: validator.requiredClaims,
			optionalClaims: validator.optionalClaims,
		}

		// Test with all claims matching
		claims, err := mockValidator.ValidateToken(ctx, tokenStr)
		assert.NoError(t, err)
		assert.Equal(t, "test-subject", claims["sub"])
		assert.Equal(t, "admin", claims["role"])
		assert.Equal(t, "engineering", claims["department"])
	})
}

// TestVerifyClaims tests the claim verification functionality
func TestVerifyClaims(t *testing.T) {
	validator := &OIDCTokenValidator{}

	t.Run("Required claims", func(t *testing.T) {
		claims := jwt.MapClaims{
			"role":       "admin",
			"department": "engineering",
		}

		// All required claims present and matching
		err := validator.verifyClaims(claims, map[string]string{
			"role": "admin",
		}, true)
		assert.NoError(t, err)

		// Required claim missing
		err = validator.verifyClaims(claims, map[string]string{
			"permission": "read",
		}, true)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrClaimsMismatch)

		// Required claim has wrong value
		err = validator.verifyClaims(claims, map[string]string{
			"role": "user",
		}, true)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrClaimsMismatch)
	})

	t.Run("Optional claims", func(t *testing.T) {
		claims := jwt.MapClaims{
			"role":       "admin",
			"department": "engineering",
		}

		// Optional claim present and matching
		err := validator.verifyClaims(claims, map[string]string{
			"department": "engineering",
		}, false)
		assert.NoError(t, err)

		// Optional claim missing (should not error)
		err = validator.verifyClaims(claims, map[string]string{
			"permission": "read",
		}, false)
		assert.NoError(t, err)

		// Optional claim has wrong value
		err = validator.verifyClaims(claims, map[string]string{
			"department": "finance",
		}, false)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrClaimsMismatch)
	})
}
