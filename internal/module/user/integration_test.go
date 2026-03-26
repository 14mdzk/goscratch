//go:build integration

package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/14mdzk/goscratch/internal/platform/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// userTestEnv holds shared test infrastructure for user integration tests.
type userTestEnv struct {
	app          interface{ Test(*http.Request, ...int) (*http.Response, error) }
	pgConn       string
	accessToken  string
	cleanupFuncs []func()
}

func setupUserTestEnv(t *testing.T) *userTestEnv {
	t.Helper()
	ctx := context.Background()

	pgConn, pgCleanup, err := testutil.StartPostgres(ctx)
	require.NoError(t, err)

	redisAddr, redisCleanup, err := testutil.StartRedis(ctx)
	require.NoError(t, err)

	app, appCleanup, err := testutil.NewTestApp(ctx, pgConn, redisAddr)
	require.NoError(t, err)

	// Create a seed user and get an access token via login
	pool, err := pgxpool.New(ctx, pgConn)
	require.NoError(t, err)

	hash, err := bcrypt.GenerateFromPassword([]byte("AdminPass123!"), bcrypt.DefaultCost)
	require.NoError(t, err)

	var adminUserID string
	err = pool.QueryRow(ctx,
		"INSERT INTO users (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id",
		"admin@test.com", string(hash), "Admin User",
	).Scan(&adminUserID)
	require.NoError(t, err)
	pool.Close()

	// Login to get access token
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "admin@test.com",
		"password": "AdminPass123!",
	})
	loginReq, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")

	loginResp, err := app.Test(loginReq, -1)
	require.NoError(t, err)
	defer loginResp.Body.Close()
	require.Equal(t, http.StatusOK, loginResp.StatusCode)

	respBody := parseJSON(t, loginResp)
	data := respBody["data"].(map[string]interface{})
	accessToken := data["access_token"].(string)

	return &userTestEnv{
		app:         app,
		pgConn:      pgConn,
		accessToken: accessToken,
		cleanupFuncs: []func(){
			appCleanup,
			redisCleanup,
			pgCleanup,
		},
	}
}

func (e *userTestEnv) cleanup() {
	for _, fn := range e.cleanupFuncs {
		fn()
	}
}

func TestCreateUser(t *testing.T) {
	env := setupUserTestEnv(t)
	defer env.cleanup()

	t.Run("create user via POST /users returns 201", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    "newuser@example.com",
			"password": "StrongPass123!",
			"name":     "New User",
		})
		req, _ := http.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+env.accessToken)

		resp, err := env.app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		respBody := parseJSON(t, resp)
		data := respBody["data"].(map[string]interface{})
		assert.Equal(t, "newuser@example.com", data["email"])
		assert.Equal(t, "New User", data["name"])
		assert.NotEmpty(t, data["id"])
	})

	t.Run("create user with duplicate email returns 409", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    "newuser@example.com",
			"password": "StrongPass123!",
			"name":     "Duplicate User",
		})
		req, _ := http.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+env.accessToken)

		resp, err := env.app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("create user without auth returns 401", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    "unauth@example.com",
			"password": "StrongPass123!",
			"name":     "Unauth User",
		})
		req, _ := http.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := env.app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

func TestGetUserByID(t *testing.T) {
	env := setupUserTestEnv(t)
	defer env.cleanup()

	// Create a user first
	createBody, _ := json.Marshal(map[string]string{
		"email":    "getuser@example.com",
		"password": "StrongPass123!",
		"name":     "Get User",
	})
	createReq, _ := http.NewRequest(http.MethodPost, "/users", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+env.accessToken)

	createResp, err := env.app.Test(createReq, -1)
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	createData := parseJSON(t, createResp)
	userID := createData["data"].(map[string]interface{})["id"].(string)

	t.Run("get user by valid ID returns 200", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/users/"+userID, nil)
		req.Header.Set("Authorization", "Bearer "+env.accessToken)

		resp, err := env.app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody := parseJSON(t, resp)
		data := respBody["data"].(map[string]interface{})
		assert.Equal(t, userID, data["id"])
		assert.Equal(t, "getuser@example.com", data["email"])
		assert.Equal(t, "Get User", data["name"])
	})

	t.Run("get user by nonexistent ID returns 404", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/users/00000000-0000-0000-0000-000000000000", nil)
		req.Header.Set("Authorization", "Bearer "+env.accessToken)

		resp, err := env.app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestListUsers(t *testing.T) {
	env := setupUserTestEnv(t)
	defer env.cleanup()

	// Create a few users for pagination
	for i := 0; i < 3; i++ {
		body, _ := json.Marshal(map[string]string{
			"email":    fmt.Sprintf("listuser%d@example.com", i),
			"password": "StrongPass123!",
			"name":     fmt.Sprintf("List User %d", i),
		})
		req, _ := http.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+env.accessToken)

		resp, err := env.app.Test(req, -1)
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	t.Run("list users returns paginated results", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/users?limit=10", nil)
		req.Header.Set("Authorization", "Bearer "+env.accessToken)

		resp, err := env.app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody := parseJSON(t, resp)
		data, ok := respBody["data"].([]interface{})
		require.True(t, ok, "data should be an array")
		// 3 created + 1 admin seed = 4 users
		assert.GreaterOrEqual(t, len(data), 4)
	})

	t.Run("list users with small limit paginates", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/users?limit=2", nil)
		req.Header.Set("Authorization", "Bearer "+env.accessToken)

		resp, err := env.app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody := parseJSON(t, resp)
		data, ok := respBody["data"].([]interface{})
		require.True(t, ok, "data should be an array")
		assert.LessOrEqual(t, len(data), 2)

		// Should have pagination info
		pagination, ok := respBody["pagination"].(map[string]interface{})
		require.True(t, ok, "should have pagination metadata")
		assert.NotNil(t, pagination)
	})
}

func TestUpdateUser(t *testing.T) {
	env := setupUserTestEnv(t)
	defer env.cleanup()

	// Create a user
	createBody, _ := json.Marshal(map[string]string{
		"email":    "updateuser@example.com",
		"password": "StrongPass123!",
		"name":     "Update User",
	})
	createReq, _ := http.NewRequest(http.MethodPost, "/users", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+env.accessToken)

	createResp, err := env.app.Test(createReq, -1)
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	createData := parseJSON(t, createResp)
	userID := createData["data"].(map[string]interface{})["id"].(string)

	t.Run("update user name returns 200", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"name": "Updated Name",
		})
		req, _ := http.NewRequest(http.MethodPut, "/users/"+userID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+env.accessToken)

		resp, err := env.app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody := parseJSON(t, resp)
		data := respBody["data"].(map[string]interface{})
		assert.Equal(t, "Updated Name", data["name"])
	})

	t.Run("update user email returns 200", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email": "updated@example.com",
		})
		req, _ := http.NewRequest(http.MethodPut, "/users/"+userID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+env.accessToken)

		resp, err := env.app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody := parseJSON(t, resp)
		data := respBody["data"].(map[string]interface{})
		assert.Equal(t, "updated@example.com", data["email"])
	})
}

func TestDeleteUser(t *testing.T) {
	env := setupUserTestEnv(t)
	defer env.cleanup()

	// Create a user
	createBody, _ := json.Marshal(map[string]string{
		"email":    "deleteuser@example.com",
		"password": "StrongPass123!",
		"name":     "Delete User",
	})
	createReq, _ := http.NewRequest(http.MethodPost, "/users", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+env.accessToken)

	createResp, err := env.app.Test(createReq, -1)
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	createData := parseJSON(t, createResp)
	userID := createData["data"].(map[string]interface{})["id"].(string)

	t.Run("delete user returns 204 (soft delete)", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, "/users/"+userID, nil)
		req.Header.Set("Authorization", "Bearer "+env.accessToken)

		resp, err := env.app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	t.Run("deleted user is still retrievable but deactivated", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/users/"+userID, nil)
		req.Header.Set("Authorization", "Bearer "+env.accessToken)

		resp, err := env.app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody := parseJSON(t, resp)
		data := respBody["data"].(map[string]interface{})
		assert.Equal(t, false, data["is_active"])
	})
}

// parseJSON reads and parses a JSON response body.
func parseJSON(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err, "failed to parse response: %s", string(body))
	return result
}
