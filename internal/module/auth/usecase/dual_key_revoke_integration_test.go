//go:build integration

package usecase_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/14mdzk/goscratch/internal/platform/testutil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// tokenHash mirrors the key-derivation logic in auth_usecase.go without
// importing that internal package. Duplicated deliberately: a future rename
// of the production function does NOT silently let this test pass — the test
// must re-derive the key shape from first principles.
func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func tokLookupKey(token string) string {
	return fmt.Sprintf("refresh:tok:%s", tokenHash(token))
}

func userIdxKey(userID, token string) string {
	return fmt.Sprintf("refresh:user:%s:%s", userID, tokenHash(token))
}

// integrationJWTClaims mirrors the shape expected by the auth middleware.
// Must stay in sync with the jwtClaims struct in auth_usecase.go and
// testJWTClaims in testutil/testapp.go.
type integrationJWTClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

// generateValidAccessToken creates a JWT that the application's Auth
// middleware will accept. The middleware uses DefaultAuthConfig which
// hardcodes issuer="goscratch" and audience="goscratch-api"; these must
// be present in the token for the middleware to allow the request through.
func generateValidAccessToken(t *testing.T, userID, email, name string) string {
	t.Helper()

	now := time.Now()
	claims := integrationJWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    "goscratch",
			Audience:  jwt.ClaimStrings{"goscratch-api"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			NotBefore: jwt.NewNumericDate(now),
		},
		UserID: userID,
		Email:  email,
		Name:   name,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testutil.TestJWTSecret()))
	require.NoError(t, err)
	return signed
}

// seedUser inserts a user with a bcrypt-hashed password directly into the DB
// and returns the userID.
func seedUser(ctx context.Context, t *testing.T, connStr, email, password, name string) string {
	t.Helper()

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	var userID string
	err = pool.QueryRow(ctx,
		"INSERT INTO users (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id",
		email, string(hash), name,
	).Scan(&userID)
	require.NoError(t, err)

	return userID
}

// parseBody reads and unmarshals a JSON HTTP response body.
func parseBody(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &result),
		"unexpected non-JSON body: %s", string(raw))
	return result
}

// doRequest sends a JSON request to the Fiber test app.
// If bearerToken is non-empty, it is added as Authorization: Bearer <token>.
func doRequest(t *testing.T, app interface {
	Test(*http.Request, ...int) (*http.Response, error)
}, method, path string, body interface{}, bearerToken string) *http.Response {
	t.Helper()

	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		require.NoError(t, err)
	}

	req, err := http.NewRequest(method, path, bytes.NewReader(reqBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	return resp
}

// TestDualKeyRevokeOnPasswordChange is the end-to-end integration test for
// PR-20. It exercises the full revoke path shipped in PR-03:
//
//  1. Login → both Redis keys (refresh:tok:<sha256> and
//     refresh:user:<userID>:<sha256>) must exist with positive TTLs.
//  2. ChangePassword (via POST /users/me/password) triggers
//     authUseCase.RevokeAllForUser which deletes the per-user index prefix.
//     Both keys must be absent from Redis after the call.
//  3. A Refresh attempt with the old token must return 401.
//
// The test fails if revoke logic ever leaves either Redis key behind, or if
// the post-revoke Refresh attempt succeeds.
func TestDualKeyRevokeOnPasswordChange(t *testing.T) {
	ctx := context.Background()

	pgConn, pgCleanup, err := testutil.StartPostgres(ctx)
	require.NoError(t, err)
	defer pgCleanup()

	redisAddr, redisCleanup, err := testutil.StartRedis(ctx)
	require.NoError(t, err)
	defer redisCleanup()

	app, appCleanup, err := testutil.NewTestApp(ctx, pgConn, redisAddr)
	require.NoError(t, err)
	defer appCleanup()

	// Direct Redis client to assert key presence/TTL without going through the
	// application's port.Cache abstraction.
	rc := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rc.Close()

	const (
		email       = "revoke-integration@example.com"
		password    = "OriginalPass123!"
		newPassword = "ChangedPass456!"
		name        = "Revoke Test User"
	)

	// ---- Step 1: seed a user and log in to obtain a refresh token. ----

	userID := seedUser(ctx, t, pgConn, email, password, name)

	loginResp := doRequest(t, app, http.MethodPost, "/auth/login",
		map[string]string{"email": email, "password": password}, "")
	defer loginResp.Body.Close()
	require.Equal(t, http.StatusOK, loginResp.StatusCode, "login must succeed")

	loginData := parseBody(t, loginResp)
	tokens := loginData["data"].(map[string]interface{})
	refreshToken, _ := tokens["refresh_token"].(string)
	require.NotEmpty(t, refreshToken, "login must return a refresh_token")

	// Generate a JWT that the auth middleware accepts (issuer="goscratch",
	// audience="goscratch-api", signed with the test secret). The token
	// returned from /auth/login is issued with TestJWTConfig which lacks
	// Issuer/Audience, so we synthesize a valid one here for the auth-gated
	// ChangePassword endpoint.
	accessToken := generateValidAccessToken(t, userID, email, name)

	// ---- Step 2: assert both Redis keys exist with positive TTL. ----

	lookupKey := tokLookupKey(refreshToken)
	idxKey := userIdxKey(userID, refreshToken)

	t.Run("both Redis keys exist with positive TTL after login", func(t *testing.T) {
		ttlLookup, err := rc.TTL(ctx, lookupKey).Result()
		require.NoError(t, err)
		assert.Greater(t, ttlLookup.Seconds(), float64(0),
			"lookup key %q must have a positive TTL, got %v", lookupKey, ttlLookup)

		ttlIdx, err := rc.TTL(ctx, idxKey).Result()
		require.NoError(t, err)
		assert.Greater(t, ttlIdx.Seconds(), float64(0),
			"index key %q must have a positive TTL, got %v", idxKey, ttlIdx)

		// Both TTLs must be equal (set with the same RefreshTokenTTL value).
		// Allow 2 s for clock jitter between the two SET calls.
		delta := ttlLookup - ttlIdx
		if delta < 0 {
			delta = -delta
		}
		assert.Less(t, delta.Seconds(), float64(2),
			"lookup and index key TTLs should match within 2s, delta=%v", delta)
	})

	// ---- Step 3: change the user's password (triggers RevokeAllForUser). ----

	changeResp := doRequest(t, app, http.MethodPost, "/users/me/password",
		map[string]string{
			"current_password": password,
			"new_password":     newPassword,
		}, accessToken)
	changeBody, _ := io.ReadAll(changeResp.Body)
	changeResp.Body.Close()
	require.Equal(t, http.StatusOK, changeResp.StatusCode,
		"ChangePassword must succeed; body: %s", string(changeBody))

	// ---- Step 4: assert BOTH Redis keys are gone. ----

	t.Run("lookup key is deleted after ChangePassword", func(t *testing.T) {
		n, err := rc.Exists(ctx, lookupKey).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(0), n,
			"lookup key %q must not exist after ChangePassword", lookupKey)
	})

	t.Run("index key is deleted after ChangePassword", func(t *testing.T) {
		n, err := rc.Exists(ctx, idxKey).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(0), n,
			"index key %q must not exist after ChangePassword", idxKey)
	})

	// ---- Step 5: old refresh token must be rejected with 401. ----

	t.Run("old refresh token returns 401 after ChangePassword", func(t *testing.T) {
		refreshResp := doRequest(t, app, http.MethodPost, "/auth/refresh",
			map[string]string{"refresh_token": refreshToken}, "")
		defer refreshResp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, refreshResp.StatusCode,
			"refresh with the old token must return 401 after password change")
	})
}
