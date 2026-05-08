package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/14mdzk/goscratch/internal/platform/config"
	platformhttp "github.com/14mdzk/goscratch/internal/platform/http"
	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, serverCfg config.ServerConfig) *fiber.App {
	t.Helper()
	log := logger.New(logger.Config{Level: "error", Format: "json"})
	srv := platformhttp.NewServer(serverCfg, log, false)
	app := srv.App()
	app.Get("/ip", func(c *fiber.Ctx) error {
		return c.SendString(c.IP())
	})
	return app
}

// TestProxyHeader_TrustedProxy_UsesXFF verifies that when the remote addr is
// in the trusted-proxy CIDR, c.IP() returns the X-Forwarded-For value.
//
// Note: fiber.App.Test() uses 0.0.0.0 as the synthetic remote address, so we
// trust 0.0.0.0/0 here to exercise the code path that honours XFF.  In
// production the operator would set a tighter CIDR (e.g. the nginx egress IP).
func TestProxyHeader_TrustedProxy_UsesXFF(t *testing.T) {
	app := newTestServer(t, config.ServerConfig{
		TrustedProxies: []string{"0.0.0.0/0"},
		ProxyHeader:    "X-Forwarded-For",
	})

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.42")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body := make([]byte, 64)
	n, _ := resp.Body.Read(body)
	ip := string(body[:n])
	assert.Equal(t, "203.0.113.42", ip,
		"c.IP() should return the XFF value when request comes from a trusted proxy")
}

// TestProxyHeader_UntrustedProxy_UsesSocketAddr verifies that when no trusted
// proxies are configured, c.IP() returns the socket remote address, not the
// X-Forwarded-For header value.
func TestProxyHeader_UntrustedProxy_UsesSocketAddr(t *testing.T) {
	// No trusted proxies — XFF header must be ignored.
	app := newTestServer(t, config.ServerConfig{
		TrustedProxies: nil,
		ProxyHeader:    "",
	})

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.99")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body := make([]byte, 64)
	n, _ := resp.Body.Read(body)
	ip := string(body[:n])
	assert.NotEqual(t, "203.0.113.99", ip,
		"c.IP() should NOT return the XFF value when no trusted proxies are configured")
}
