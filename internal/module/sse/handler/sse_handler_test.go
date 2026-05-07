package handler

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/14mdzk/goscratch/internal/adapter/sse"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestApp(broker port.SSEBroker) (*fiber.App, *Handler) {
	app := fiber.New()
	h := NewHandler(broker)
	return app, h
}

func TestBroadcast(t *testing.T) {
	broker := sse.NewBroker(10)
	defer broker.Close()

	app, h := setupTestApp(broker)
	app.Post("/sse/broadcast", h.Broadcast)

	t.Run("successful broadcast", func(t *testing.T) {
		body := `{"event":"test-event","data":"hello world"}`
		req := httptest.NewRequest("POST", "/sse/broadcast", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)
		assert.True(t, result["success"].(bool))
	})

	t.Run("broadcast to topic", func(t *testing.T) {
		body := `{"event":"test-event","data":"topic data","topic":"notifications"}`
		req := httptest.NewRequest("POST", "/sse/broadcast", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("missing event type", func(t *testing.T) {
		body := `{"data":"no event"}`
		req := httptest.NewRequest("POST", "/sse/broadcast", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})

	t.Run("invalid body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/sse/broadcast", strings.NewReader("not json"))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})
}

func TestClientCount(t *testing.T) {
	broker := sse.NewBroker(10)
	defer broker.Close()

	app, h := setupTestApp(broker)
	app.Get("/sse/clients", h.ClientCount)

	t.Run("zero clients initially", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/sse/clients", nil)

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)
		assert.True(t, result["success"].(bool))

		data := result["data"].(map[string]interface{})
		assert.Equal(t, float64(0), data["count"])
	})

	t.Run("counts subscribed clients", func(t *testing.T) {
		broker.Subscribe("client-1")
		broker.Subscribe("client-2")

		req := httptest.NewRequest("GET", "/sse/clients", nil)

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		data := result["data"].(map[string]interface{})
		assert.Equal(t, float64(2), data["count"])

		// Cleanup
		broker.Unsubscribe("client-1")
		broker.Unsubscribe("client-2")
	})
}

func TestSubscribe_Unauthorized(t *testing.T) {
	broker := sse.NewBroker(10)
	defer broker.Close()

	_, h := setupTestApp(broker)

	t.Run("returns unauthorized without user_id", func(t *testing.T) {
		noAuthApp := fiber.New()
		noAuthApp.Get("/sse/subscribe", h.Subscribe)

		req := httptest.NewRequest("GET", "/sse/subscribe", nil)

		resp, err := noAuthApp.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	})
}

func TestSubscribe_StreamHeaders(t *testing.T) {
	broker := sse.NewBroker(10)

	app := fiber.New()
	h := NewHandler(broker)

	app.Get("/sse/subscribe", func(c *fiber.Ctx) error {
		c.Locals("user_id", "test-user-headers")
		return h.Subscribe(c)
	})

	// Close broker after subscription is established to end the stream. We
	// can no longer Unsubscribe by userID because the handler keys
	// subscriptions by per-connection UUID (block-ship #11/#12 fix).
	go func() {
		for i := 0; i < 50; i++ {
			if broker.ClientCount() > 0 {
				_ = broker.Close()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		_ = broker.Close()
	}()

	req := httptest.NewRequest("GET", "/sse/subscribe", nil)
	resp, err := app.Test(req, 5000)
	require.NoError(t, err)

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "keep-alive", resp.Header.Get("Connection"))
}
