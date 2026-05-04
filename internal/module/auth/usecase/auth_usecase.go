package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/14mdzk/goscratch/internal/module/auth/dto"
	userrepo "github.com/14mdzk/goscratch/internal/module/user/repository"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// authUseCase handles authentication business logic
type authUseCase struct {
	userRepo *userrepo.Repository
	cache    port.Cache
	jwtCfg   config.JWTConfig
}

// NewUseCase creates a new auth use case
func NewUseCase(userRepo *userrepo.Repository, cache port.Cache, jwtCfg config.JWTConfig) UseCase {
	return &authUseCase{
		userRepo: userRepo,
		cache:    cache,
		jwtCfg:   jwtCfg,
	}
}

// Login authenticates a user and returns tokens
func (uc *authUseCase) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
	// Get user by email
	user, err := uc.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		// Don't reveal if user exists
		return nil, apperr.ErrUnauthorized.WithMessage("Invalid email or password")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, apperr.ErrUnauthorized.WithMessage("Invalid email or password")
	}

	// Generate tokens
	accessToken, err := uc.generateAccessToken(user.ID.String(), user.Email, user.Name)
	if err != nil {
		return nil, apperr.Internalf("failed to generate access token")
	}

	refreshToken, err := uc.generateRefreshToken()
	if err != nil {
		return nil, apperr.Internalf("failed to generate refresh token")
	}

	// Store refresh token in cache (if enabled)
	refreshKey := "refresh:" + refreshToken
	_ = uc.cache.Set(ctx, refreshKey, []byte(user.ID.String()), uc.jwtCfg.RefreshTokenDuration())

	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    uc.jwtCfg.AccessTokenTTL * 60, // Convert minutes to seconds
		TokenType:    "Bearer",
		UserID:       user.ID.String(),
	}, nil
}

// Refresh refreshes an access token using a refresh token
func (uc *authUseCase) Refresh(ctx context.Context, req dto.RefreshRequest) (*dto.RefreshResponse, error) {
	// Validate refresh token
	refreshKey := "refresh:" + req.RefreshToken
	userIDBytes, err := uc.cache.Get(ctx, refreshKey)
	if err != nil {
		return nil, apperr.ErrUnauthorized.WithMessage("Invalid or expired refresh token")
	}

	userID := string(userIDBytes)

	// Get user
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, apperr.ErrUnauthorized.WithMessage("User not found")
	}

	// Revoke old refresh token
	_ = uc.cache.Delete(ctx, refreshKey)

	// Generate new tokens
	accessToken, err := uc.generateAccessToken(user.ID.String(), user.Email, user.Name)
	if err != nil {
		return nil, apperr.Internalf("failed to generate access token")
	}

	newRefreshToken, err := uc.generateRefreshToken()
	if err != nil {
		return nil, apperr.Internalf("failed to generate refresh token")
	}

	// Store new refresh token
	newRefreshKey := "refresh:" + newRefreshToken
	_ = uc.cache.Set(ctx, newRefreshKey, []byte(user.ID.String()), uc.jwtCfg.RefreshTokenDuration())

	return &dto.RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    uc.jwtCfg.AccessTokenTTL * 60,
		TokenType:    "Bearer",
	}, nil
}

// Logout invalidates a refresh token
func (uc *authUseCase) Logout(ctx context.Context, refreshToken string) error {
	refreshKey := "refresh:" + refreshToken
	_ = uc.cache.Delete(ctx, refreshKey)

	return nil
}

// generateAccessToken generates a JWT access token
func (uc *authUseCase) generateAccessToken(userID, email, name string) (string, error) {
	now := time.Now()

	issuer := uc.jwtCfg.Issuer
	if issuer == "" {
		issuer = "goscratch"
	}
	audience := uc.jwtCfg.Audience
	if audience == "" {
		audience = "goscratch-api"
	}

	claims := middleware.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(uc.jwtCfg.AccessTokenDuration())),
			NotBefore: jwt.NewNumericDate(now),
		},
		UserID: userID,
		Email:  email,
		Name:   name,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(uc.jwtCfg.Secret))
}

// generateRefreshToken generates a random refresh token
func (uc *authUseCase) generateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
