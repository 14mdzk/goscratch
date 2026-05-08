package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/14mdzk/goscratch/internal/module/auth/dto"
	userdomain "github.com/14mdzk/goscratch/internal/module/user/domain"
	"github.com/14mdzk/goscratch/internal/platform/config"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// UserRepo is the narrow repository interface that the auth usecase depends on.
// Using an interface makes the usecase independently testable without a live
// database; the concrete *userrepo.Repository satisfies it. Exported so the
// auth.Module can accept any implementation (including the one created by the
// user module) without importing the concrete type, avoiding a second DB pool
// connection for the same pool (audit finding: auth/module.go:20).
type UserRepo interface {
	GetByEmail(ctx context.Context, email string) (*userdomain.User, error)
	GetByID(ctx context.Context, id string) (*userdomain.User, error)
}

// userLookup is an internal alias kept for backward compat with the field type.
type userLookup = UserRepo

// authUseCase handles authentication business logic
type authUseCase struct {
	userRepo userLookup
	cache    port.Cache
	jwtCfg   config.JWTConfig
}

// NewUseCase creates a new auth use case.
// userRepo accepts any value satisfying userLookup (GetByEmail + GetByID),
// which the concrete *userrepo.Repository satisfies. Accepting the interface
// allows the caller (auth.Module) to inject the same repository instance
// already created by the user module, avoiding a second pool connection.
func NewUseCase(userRepo userLookup, cache port.Cache, jwtCfg config.JWTConfig) UseCase {
	return &authUseCase{
		userRepo: userRepo,
		cache:    cache,
		jwtCfg:   jwtCfg,
	}
}

// tokenHash returns the full SHA-256 hex string (64 chars) of the token.
// Using the full hash avoids the birthday-bound truncation weakness of a
// 16-char prefix.
func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// tokLookupKey returns the lookup key: refresh:tok:<sha256-hex(token)>
// Value stored: userID.  Used by Refresh to translate a token into a userID
// without any client-supplied hint.
func tokLookupKey(token string) string {
	return fmt.Sprintf("refresh:tok:%s", tokenHash(token))
}

// userIdxKey returns the per-user index key:
// refresh:user:<userID>:<sha256-hex(token)>
// Value stored: "1".  Used by RevokeAllForUser to delete all tokens for a user
// via prefix iteration.
func userIdxKey(userID, token string) string {
	return fmt.Sprintf("refresh:user:%s:%s", userID, tokenHash(token))
}

// Login authenticates a user and returns tokens.
// Dual-key write: both the lookup key and the per-user index key are stored.
// If either write fails the partner key is deleted best-effort and the login
// is rejected (fail-closed semantics).
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

	ttl := uc.jwtCfg.RefreshTokenDuration()
	lookupKey := tokLookupKey(refreshToken)
	idxKey := userIdxKey(user.ID.String(), refreshToken)

	// Write lookup key first.
	if err := uc.cache.Set(ctx, lookupKey, []byte(user.ID.String()), ttl); err != nil {
		return nil, apperr.Internalf("auth: cache unavailable, cannot issue refresh token")
	}

	// Write per-user index key. On failure, delete the already-written lookup
	// key best-effort to avoid an orphan, then return.
	if err := uc.cache.Set(ctx, idxKey, []byte("1"), ttl); err != nil {
		_ = uc.cache.Delete(ctx, lookupKey)
		return nil, apperr.Internalf("auth: cache unavailable, cannot issue refresh token")
	}

	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    uc.jwtCfg.AccessTokenTTL * 60, // Convert minutes to seconds
		TokenType:    "Bearer",
		UserID:       user.ID.String(),
	}, nil
}

// Refresh refreshes an access token using a refresh token.
// The client POSTs only the opaque refresh_token; the server resolves the
// userID from the lookup key without any client-supplied hint.
//
// Dual-key validation: BOTH the lookup key AND the per-user index key must
// exist. The index key is the revocation gate: RevokeAllForUser (called by
// ChangePassword) deletes only the index keys, leaving orphaned lookup keys
// until TTL expiry. Checking the index key here means a password change
// immediately invalidates all sessions even if the lookup key is still cached.
func (uc *authUseCase) Refresh(ctx context.Context, req dto.RefreshRequest) (*dto.RefreshResponse, error) {
	lookupKey := tokLookupKey(req.RefreshToken)

	userIDBytes, err := uc.cache.Get(ctx, lookupKey)
	if err != nil {
		// Cache miss or error — both treated as invalid/expired.
		return nil, apperr.ErrUnauthorized.WithMessage("Invalid or expired refresh token")
	}

	userID := string(userIDBytes)

	// Verify the per-user index key also exists. Absence means the token was
	// revoked (e.g., by ChangePassword → RevokeAllForUser) even though the
	// lookup key has not yet TTL-expired. Use the same error message to avoid
	// an existence oracle.
	idxKey := userIdxKey(userID, req.RefreshToken)
	if _, err := uc.cache.Get(ctx, idxKey); err != nil {
		return nil, apperr.ErrUnauthorized.WithMessage("Invalid or expired refresh token")
	}

	// Get user
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, apperr.ErrUnauthorized.WithMessage("User not found")
	}

	// Revoke old token: delete both keys (idxKey was validated above).
	_ = uc.cache.Delete(ctx, lookupKey)
	_ = uc.cache.Delete(ctx, idxKey)

	// Generate new tokens
	accessToken, err := uc.generateAccessToken(user.ID.String(), user.Email, user.Name)
	if err != nil {
		return nil, apperr.Internalf("failed to generate access token")
	}

	newRefreshToken, err := uc.generateRefreshToken()
	if err != nil {
		return nil, apperr.Internalf("failed to generate refresh token")
	}

	// Issue new dual keys — fail-closed.
	ttl := uc.jwtCfg.RefreshTokenDuration()
	newLookupKey := tokLookupKey(newRefreshToken)
	newIdxKey := userIdxKey(user.ID.String(), newRefreshToken)

	if err := uc.cache.Set(ctx, newLookupKey, []byte(user.ID.String()), ttl); err != nil {
		return nil, apperr.Internalf("auth: cache unavailable, cannot issue refresh token")
	}
	if err := uc.cache.Set(ctx, newIdxKey, []byte("1"), ttl); err != nil {
		_ = uc.cache.Delete(ctx, newLookupKey)
		return nil, apperr.Internalf("auth: cache unavailable, cannot issue refresh token")
	}

	return &dto.RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    uc.jwtCfg.AccessTokenTTL * 60,
		TokenType:    "Bearer",
	}, nil
}

// Logout invalidates a refresh token.
//
// Only the caller's own token is invalidated: we first resolve the lookup key
// to confirm the stored userID matches callerID before deleting either key.
// If the lookup key is missing or resolves to a different user, we return
// success silently — this avoids a token-existence oracle (an attacker who
// knows another user's refresh token cannot use their own JWT to probe whether
// that token is still live).
func (uc *authUseCase) Logout(ctx context.Context, callerID, refreshToken string) error {
	lookupKey := tokLookupKey(refreshToken)

	storedUserID, err := uc.cache.Get(ctx, lookupKey)
	if err != nil {
		// Miss (already revoked or never existed) — silent success to avoid oracle.
		return nil
	}

	if string(storedUserID) != callerID {
		// Token belongs to a different user — silent success to avoid oracle.
		// This prevents an attacker who has another user's refresh token from
		// using their own valid JWT to confirm whether that token is still live.
		return nil
	}

	idxKey := userIdxKey(callerID, refreshToken)
	_ = uc.cache.Delete(ctx, lookupKey)
	_ = uc.cache.Delete(ctx, idxKey)
	return nil
}

// RevokeAllForUser implements the Revoker interface. It scans all per-user
// index keys for userID and for each match deletes both the index key and its
// corresponding lookup key.
func (uc *authUseCase) RevokeAllForUser(ctx context.Context, userID string) error {
	prefix := fmt.Sprintf("refresh:user:%s:", userID)
	return uc.cache.DeleteByPrefix(ctx, prefix)
}

// jwtClaims is a local JWT-lib struct used only for signing access tokens.
// It mirrors the shape that middleware.parseToken expects so both sides of
// the JWT boundary stay in sync. The domain Claims type (authdomain.Claims)
// is the public contract; this struct is an implementation detail.
type jwtClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

// generateAccessToken generates a JWT access token
func (uc *authUseCase) generateAccessToken(userID, email, name string) (string, error) {
	now := time.Now()

	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    uc.jwtCfg.Issuer,
			Audience:  jwt.ClaimStrings{uc.jwtCfg.Audience},
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
