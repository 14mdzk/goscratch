package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testJWTSecret = "test-secret-key-for-unit-tests"

func generateTestToken(t *testing.T, secret string, claims Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	tokenStr, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return tokenStr
}

func validClaims() Claims {
	return Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "goscratch",
			Audience:  jwt.ClaimStrings{"goscratch-api"},
		},
		UserID: "user-123",
		Email:  "test@example.com",
		Name:   "Test User",
	}
}

func TestAuth_ValidJWTInHeader(t *testing.T) {
	app := fiber.New()
	cfg := DefaultAuthConfig(testJWTSecret)

	var capturedUserID string
	var capturedClaims *Claims
	var capturedCtx context.Context

	app.Use(Auth(cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		capturedClaims = GetClaims(c)
		capturedUserID = GetUserID(c)
		capturedCtx = c.UserContext()
		return c.SendStatus(fiber.StatusOK)
	})

	token := generateTestToken(t, testJWTSecret, validClaims())
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Test-Agent/1.0")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.NotNil(t, capturedClaims)
	assert.Equal(t, "user-123", capturedUserID)
	assert.Equal(t, "test@example.com", capturedClaims.Email)
	assert.Equal(t, "Test User", capturedClaims.Name)

	// Auth middleware writes typed context keys for downstream auditor/logger.
	require.NotNil(t, capturedCtx)
	assert.Equal(t, "user-123", capturedCtx.Value(logger.UserIDKey))
	assert.Equal(t, "Test-Agent/1.0", capturedCtx.Value(logger.UserAgentKey))
	// IPAddressKey is set from c.IP(); under app.Test with httptest the
	// remote addr is non-empty but the exact value is not asserted to keep
	// the test stable across Fiber versions.
	assert.NotEmpty(t, capturedCtx.Value(logger.IPAddressKey))
}

func TestAuth_ValidJWTInCookie(t *testing.T) {
	app := fiber.New()
	cfg := AuthConfig{
		JWTSecret:   testJWTSecret,
		JWTIssuer:   "goscratch",
		JWTAudience: "goscratch-api",
		TokenLookup: "cookie:token",
		ContextKey:  "user",
	}

	var capturedUserID string

	app.Use(Auth(cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		capturedUserID = GetUserID(c)
		return c.SendStatus(fiber.StatusOK)
	})

	token := generateTestToken(t, testJWTSecret, validClaims())
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "user-123", capturedUserID)
}

func TestAuth_MissingToken(t *testing.T) {
	app := fiber.New()
	cfg := DefaultAuthConfig(testJWTSecret)

	app.Use(Auth(cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_MalformedAuthorizationHeader(t *testing.T) {
	app := fiber.New()
	cfg := DefaultAuthConfig(testJWTSecret)

	app.Use(Auth(cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Token without "Bearer " prefix - extractToken returns raw value,
	// which is not a valid JWT, so parseToken will fail.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "not-a-valid-jwt-token")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_ExpiredJWT(t *testing.T) {
	app := fiber.New()
	cfg := DefaultAuthConfig(testJWTSecret)

	app.Use(Auth(cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "goscratch",
			Audience:  jwt.ClaimStrings{"goscratch-api"},
		},
		UserID: "user-123",
	}
	token := generateTestToken(t, testJWTSecret, claims)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "Token has expired")
}

func TestAuth_InvalidSignature(t *testing.T) {
	app := fiber.New()
	cfg := DefaultAuthConfig(testJWTSecret)

	app.Use(Auth(cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Sign with a different secret
	token := generateTestToken(t, "wrong-secret", validClaims())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestOptionalAuth_ValidJWT(t *testing.T) {
	app := fiber.New()
	cfg := DefaultAuthConfig(testJWTSecret)

	var capturedClaims *Claims

	app.Use(OptionalAuth(cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		capturedClaims = GetClaims(c)
		return c.SendStatus(fiber.StatusOK)
	})

	token := generateTestToken(t, testJWTSecret, validClaims())
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.NotNil(t, capturedClaims)
	assert.Equal(t, "user-123", capturedClaims.UserID)
}

func TestOptionalAuth_MissingToken(t *testing.T) {
	app := fiber.New()
	cfg := DefaultAuthConfig(testJWTSecret)

	var capturedClaims *Claims
	handlerCalled := false

	app.Use(OptionalAuth(cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		handlerCalled = true
		capturedClaims = GetClaims(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.True(t, handlerCalled)
	assert.Nil(t, capturedClaims)
}

func TestOptionalAuth_InvalidToken(t *testing.T) {
	app := fiber.New()
	cfg := DefaultAuthConfig(testJWTSecret)

	var capturedClaims *Claims
	handlerCalled := false

	app.Use(OptionalAuth(cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		handlerCalled = true
		capturedClaims = GetClaims(c)
		return c.SendStatus(fiber.StatusOK)
	})

	token := generateTestToken(t, "wrong-secret", validClaims())
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.True(t, handlerCalled)
	assert.Nil(t, capturedClaims)
}

func TestGetClaims_NoClaims(t *testing.T) {
	app := fiber.New()

	var result *Claims

	app.Get("/test", func(c *fiber.Ctx) error {
		result = GetClaims(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestGetUserID_NoUserID(t *testing.T) {
	app := fiber.New()

	var result string

	app.Get("/test", func(c *fiber.Ctx) error {
		result = GetUserID(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

// TestParseToken_StrictIssAud verifies that parseToken rejects tokens when the
// server config has empty issuer or audience (should-fix: middleware/auth.go:129).
func TestParseToken_StrictIssAud(t *testing.T) {
	validToken := generateTestToken(t, testJWTSecret, validClaims())

	t.Run("empty issuer in config rejects token", func(t *testing.T) {
		_, err := parseToken(validToken, testJWTSecret, "", "goscratch-api")
		assert.Error(t, err)
	})

	t.Run("empty audience in config rejects token", func(t *testing.T) {
		_, err := parseToken(validToken, testJWTSecret, "goscratch", "")
		assert.Error(t, err)
	})

	t.Run("both empty rejects token", func(t *testing.T) {
		_, err := parseToken(validToken, testJWTSecret, "", "")
		assert.Error(t, err)
	})

	t.Run("correct issuer and audience accepts token", func(t *testing.T) {
		claims, err := parseToken(validToken, testJWTSecret, "goscratch", "goscratch-api")
		assert.NoError(t, err)
		assert.Equal(t, "user-123", claims.UserID)
	})
}

// TestAuth_RejectsWhenIssuerOrAudienceEmpty verifies the Auth middleware itself
// refuses requests when its config has empty issuer/audience.
func TestAuth_RejectsWhenIssuerOrAudienceEmpty(t *testing.T) {
	app := fiber.New()
	cfg := AuthConfig{
		JWTSecret:   testJWTSecret,
		JWTIssuer:   "", // intentionally empty
		JWTAudience: "goscratch-api",
		TokenLookup: "header:Authorization",
		ContextKey:  "user",
	}

	app.Use(Auth(cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	token := generateTestToken(t, testJWTSecret, validClaims())
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}
