package middleware

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/14mdzk/goscratch/pkg/response"
)

// AuthConfig holds authentication middleware configuration
type AuthConfig struct {
	JWTSecret    string
	TokenLookup  string // "header:Authorization" or "cookie:token"
	ContextKey   string // Key to store user claims in context
	ErrorHandler fiber.ErrorHandler
}

// DefaultAuthConfig returns default authentication configuration
func DefaultAuthConfig(jwtSecret string) AuthConfig {
	return AuthConfig{
		JWTSecret:   jwtSecret,
		TokenLookup: "header:Authorization",
		ContextKey:  "user",
	}
}

// Claims represents JWT claims
type Claims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
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
		claims, err := parseToken(token, cfg.JWTSecret)
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				return response.Unauthorized(c, "Token has expired")
			}
			return response.Unauthorized(c, "Invalid token")
		}

		// Store claims in context
		c.Locals(cfg.ContextKey, claims)
		c.Locals("user_id", claims.UserID)

		// Add to user context for logger
		ctx := c.UserContext()
		ctx = setContextValue(ctx, logger.UserIDKey, claims.UserID)
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

		claims, err := parseToken(token, cfg.JWTSecret)
		if err != nil {
			return c.Next() // Invalid token, continue without auth
		}

		c.Locals(cfg.ContextKey, claims)
		c.Locals("user_id", claims.UserID)

		ctx := c.UserContext()
		ctx = setContextValue(ctx, logger.UserIDKey, claims.UserID)
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

// parseToken parses and validates a JWT token
func parseToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, apperr.ErrUnauthorized
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, apperr.ErrUnauthorized
	}

	return claims, nil
}

// GetClaims retrieves the claims from context
func GetClaims(c *fiber.Ctx) *Claims {
	if claims, ok := c.Locals("user").(*Claims); ok {
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
