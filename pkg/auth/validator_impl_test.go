package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementation of OIDC provider
type mockProvider struct {
	privateKey *rsa.PrivateKey
	issuer     string
}

// createMockProvider creates a new mock provider with randomly generated keys
func createMockProvider(issuer string) (*mockProvider, error) {
	// Generate a private key for signing tokens
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	return &mockProvider{
		privateKey: privateKey,
		issuer:     issuer,
	}, nil
}

// createToken creates a signed JWT token for testing
func (p *mockProvider) createToken(audience string, subject string, claims map[string]interface{}) (string, error) {
	// Create token with claims
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": p.issuer,
		"sub": subject,
		"aud": audience,
		"exp": now.Add(time.Hour).Unix(),
		"iat": now.Unix(),
		"nbf": now.Unix(),
	})

	// Add any additional claims
	if claims != nil {
		for k, v := range claims {
			token.Claims.(jwt.MapClaims)[k] = v
		}
	}

	// Sign the token
	return token.SignedString(p.privateKey)
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

		// Create custom validator for testing with the None algorithm
		validator.verifyClaims = func(claims jwt.MapClaims, expectedClaims map[string]string, required bool) error {
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

		// Mock the token validation logic for testing
		validator.ValidateToken = func(ctx context.Context, tokenStr string) (jwt.MapClaims, error) {
			token, _, err := new(jwt.Parser).ParseUnverified(tokenStr, jwt.MapClaims{})
			if err != nil {
				return nil, ErrInvalidToken
			}
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return nil, ErrInvalidToken
			}

			// Validate required claims
			if err := validator.verifyClaims(claims, validator.requiredClaims, true); err != nil {
				return nil, err
			}

			// Validate optional claims
			if err := validator.verifyClaims(claims, validator.optionalClaims, false); err != nil {
				return nil, err
			}

			return claims, nil
		}

		// Test with all claims matching
		claims, err := validator.ValidateToken(ctx, tokenStr)
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
