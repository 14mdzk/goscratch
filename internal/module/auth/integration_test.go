//go:build integration

package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/14mdzk/goscratch/internal/platform/testutil"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// seedTestUser inserts a user directly into the database for auth tests.
func seedTestUser(ctx context.Context, t *testing.T, connStr, email, password, name string) string {
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

func TestAuthLoginFlow(t *testing.T) {
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

	// Seed a test user
	testEmail := "authtest@example.com"
	testPassword := "SecurePass123!"
	_ = seedTestUser(ctx, t, pgConn, testEmail, testPassword, "Auth Test User")

	t.Run("login with valid credentials returns tokens", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    testEmail,
			"password": testPassword,
		})
		req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody := parseResponse(t, resp)
		data := respBody["data"].(map[string]interface{})
		assert.NotEmpty(t, data["access_token"])
		assert.NotEmpty(t, data["refresh_token"])
		assert.Equal(t, "Bearer", data["token_type"])
	})

	t.Run("login with invalid password returns 401", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    testEmail,
			"password": "WrongPassword",
		})
		req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("login with nonexistent email returns 401", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    "nobody@example.com",
			"password": "whatever",
		})
		req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

func TestAuthRefreshFlow(t *testing.T) {
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

	// Seed and login
	testEmail := "refresh@example.com"
	testPassword := "SecurePass123!"
	_ = seedTestUser(ctx, t, pgConn, testEmail, testPassword, "Refresh Test User")

	// Login to get tokens
	loginBody, _ := json.Marshal(map[string]string{
		"email":    testEmail,
		"password": testPassword,
	})
	loginReq, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")

	loginResp, err := app.Test(loginReq, -1)
	require.NoError(t, err)
	defer loginResp.Body.Close()
	require.Equal(t, http.StatusOK, loginResp.StatusCode)

	loginData := parseResponse(t, loginResp)
	tokens := loginData["data"].(map[string]interface{})
	refreshToken := tokens["refresh_token"].(string)

	t.Run("refresh with valid token returns new tokens", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"refresh_token": refreshToken,
		})
		req, _ := http.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody := parseResponse(t, resp)
		data := respBody["data"].(map[string]interface{})
		assert.NotEmpty(t, data["access_token"])
		assert.NotEmpty(t, data["refresh_token"])
		// New refresh token should be different from the old one
		assert.NotEqual(t, refreshToken, data["refresh_token"])
	})

	t.Run("refresh with already-used token returns 401", func(t *testing.T) {
		// The old refresh token was consumed by the previous test
		body, _ := json.Marshal(map[string]string{
			"refresh_token": refreshToken,
		})
		req, _ := http.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

func TestAuthLogoutFlow(t *testing.T) {
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

	// Seed and login
	testEmail := "logout@example.com"
	testPassword := "SecurePass123!"
	_ = seedTestUser(ctx, t, pgConn, testEmail, testPassword, "Logout Test User")

	loginBody, _ := json.Marshal(map[string]string{
		"email":    testEmail,
		"password": testPassword,
	})
	loginReq, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")

	loginResp, err := app.Test(loginReq, -1)
	require.NoError(t, err)
	defer loginResp.Body.Close()
	require.Equal(t, http.StatusOK, loginResp.StatusCode)

	loginData := parseResponse(t, loginResp)
	tokens := loginData["data"].(map[string]interface{})
	refreshToken := tokens["refresh_token"].(string)

	t.Run("logout invalidates refresh token", func(t *testing.T) {
		// Logout
		body, _ := json.Marshal(map[string]string{
			"refresh_token": refreshToken,
		})
		req, _ := http.NewRequest(http.MethodPost, "/auth/logout", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Try to refresh with the invalidated token — should fail
		refreshBody, _ := json.Marshal(map[string]string{
			"refresh_token": refreshToken,
		})
		refreshReq, _ := http.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(refreshBody))
		refreshReq.Header.Set("Content-Type", "application/json")

		refreshResp, err := app.Test(refreshReq, -1)
		require.NoError(t, err)
		defer refreshResp.Body.Close()

		assert.Equal(t, fiber.StatusUnauthorized, refreshResp.StatusCode)
	})
}

// parseResponse reads and parses a JSON response body.
func parseResponse(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err, "failed to parse response: %s", string(body))
	return result
}
