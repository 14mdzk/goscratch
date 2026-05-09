//go:build integration

package health_test

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/14mdzk/goscratch/internal/platform/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthEndpoint(t *testing.T) {
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

	t.Run("GET /health returns 200 (liveness alias)", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/health", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"status":"alive"`)
	})

	t.Run("GET /healthz/live returns 200", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/healthz/live", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"status":"alive"`)
	})

	t.Run("GET /healthz/ready returns 200 on healthy stack", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/healthz/ready", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"status":"ready"`)
		assert.Contains(t, string(body), `"database":"ok"`)
	})
}
