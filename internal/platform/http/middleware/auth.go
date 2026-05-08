package middleware

import (
	"errors"
	"strings"

	authdomain "github.com/14mdzk/goscratch/internal/module/auth/domain"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/14mdzk/goscratch/pkg/response"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// AuthConfig holds authentication middleware configuration
type AuthConfig struct {
	JWTSecret    string
	JWTIssuer    string
	JWTAudience  string
	TokenLookup  string // "header:Authorization" or "cookie:token"
	ContextKey   string // Key to store user claims in context
	ErrorHandler fiber.ErrorHandler
}

// DefaultAuthConfig returns default authentication configuration
func DefaultAuthConfig(jwtSecret string) AuthConfig {
	return AuthConfig{
		JWTSecret:   jwtSecret,
		JWTIssuer:   "goscratch",
		JWTAudience: "goscratch-api",
		TokenLookup: "header:Authorization",
		ContextKey:  "user",
	}
}

// Claims is the JWT-library bound claims struct used only inside this
// package for token parsing and signing. All other code uses authdomain.Claims.
type Claims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

// toDomainClaims maps JWT library claims to the domain Claims type.
func toDomainClaims(c *Claims) *authdomain.Claims {
	dc := &authdomain.Claims{
		Subject: c.Subject,
		UserID:  c.UserID,
		Email:   c.Email,
		Name:    c.Name,
		Issuer:  c.Issuer,
	}
	if c.Audience != nil {
		dc.Audience = []string(c.Audience)
	}
	if c.ExpiresAt != nil {
		dc.ExpiresAt = c.ExpiresAt.Time
	}
	if c.IssuedAt != nil {
		dc.IssuedAt = c.IssuedAt.Time
	}
	if c.NotBefore != nil {
		dc.NotBefore = c.NotBefore.Time
	}
	return dc
}

// Auth returns an authentication middleware
func Auth(cfg AuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract token
		token, err := extractToken(c, cfg.TokenLookup)
		if err != nil {
			return response.Unauthorized(c, "Missing or invalid token")
		}

		// Parse and validate token
		raw, err := parseToken(token, cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience)
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				return response.Unauthorized(c, "Token has expired")
			}
			return response.Unauthorized(c, "Invalid token")
		}

		claims := toDomainClaims(raw)

		// Store domain claims in context
		c.Locals(cfg.ContextKey, claims)
		c.Locals("user_id", claims.UserID)

		// Add to user context for logger and auditor
		ctx := c.UserContext()
		ctx = setContextValue(ctx, logger.UserIDKey, claims.UserID)
		ctx = setContextValue(ctx, logger.IPAddressKey, c.IP())
		ctx = setContextValue(ctx, logger.UserAgentKey, c.Get("User-Agent"))
		c.SetUserContext(ctx)

		return c.Next()
	}
}

// OptionalAuth is like Auth but doesn't fail if no token is present
func OptionalAuth(cfg AuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token, err := extractToken(c, cfg.TokenLookup)
		if err != nil || token == "" {
			return c.Next()
		}

		raw, err := parseToken(token, cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience)
		if err != nil {
			return c.Next() // Invalid token, continue without auth
		}

		claims := toDomainClaims(raw)

		c.Locals(cfg.ContextKey, claims)
		c.Locals("user_id", claims.UserID)

		ctx := c.UserContext()
		ctx = setContextValue(ctx, logger.UserIDKey, claims.UserID)
		ctx = setContextValue(ctx, logger.IPAddressKey, c.IP())
		ctx = setContextValue(ctx, logger.UserAgentKey, c.Get("User-Agent"))
		c.SetUserContext(ctx)

		return c.Next()
	}
}

// extractToken extracts the token from the request
func extractToken(c *fiber.Ctx, lookup string) (string, error) {
	parts := strings.Split(lookup, ":")
	if len(parts) != 2 {
		return "", apperr.ErrBadRequest
	}

	switch parts[0] {
	case "header":
		auth := c.Get(parts[1])
		if auth == "" {
			return "", apperr.ErrUnauthorized
		}
		// Handle "Bearer <token>" format
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer "), nil
		}
		return auth, nil
	case "cookie":
		return c.Cookies(parts[1]), nil
	default:
		return "", apperr.ErrBadRequest
	}
}

// parseToken parses and validates a JWT token, including strict issuer and
// audience checks. Both iss and aud must be non-empty in the server config; a
// token that omits or mismatches either claim is unconditionally rejected
// (should-fix: audit middleware/auth.go:129).
func parseToken(tokenString, secret, issuer, audience string) (*Claims, error) {
	// Strict: the server config must provide both iss and aud (enforced by
	// config.Validate). A call with empty issuer or audience is a programming
	// error — reject the token immediately rather than skipping validation.
	if issuer == "" || audience == "" {
		return nil, apperr.ErrUnauthorized
	}

	parserOpts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, apperr.ErrUnauthorized
		}
		return []byte(secret), nil
	}, parserOpts...)

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, apperr.ErrUnauthorized
	}

	return claims, nil
}

// GetClaims retrieves the domain claims from context
func GetClaims(c *fiber.Ctx) *authdomain.Claims {
	if claims, ok := c.Locals("user").(*authdomain.Claims); ok {
		return claims
	}
	return nil
}

// GetUserID retrieves the user ID from context
func GetUserID(c *fiber.Ctx) string {
	if userID, ok := c.Locals("user_id").(string); ok {
		return userID
	}
	return ""
}
