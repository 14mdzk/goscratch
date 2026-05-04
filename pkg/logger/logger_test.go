package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("json_format", func(t *testing.T) {
		buf := &bytes.Buffer{}
		log := New(Config{Level: "info", Format: "json", Output: buf})
		require.NotNil(t, log)

		log.Info("test message")
		assert.Contains(t, buf.String(), "test message")

		// Verify it's valid JSON
		var m map[string]any
		err := json.Unmarshal(buf.Bytes(), &m)
		assert.NoError(t, err)
	})

	t.Run("text_format", func(t *testing.T) {
		buf := &bytes.Buffer{}
		log := New(Config{Level: "info", Format: "text", Output: buf})
		require.NotNil(t, log)

		log.Info("text message")
		assert.Contains(t, buf.String(), "text message")
	})

	t.Run("debug_level", func(t *testing.T) {
		buf := &bytes.Buffer{}
		log := New(Config{Level: "debug", Format: "json", Output: buf})

		log.Debug("debug msg")
		assert.Contains(t, buf.String(), "debug msg")
	})

	t.Run("warn_level", func(t *testing.T) {
		buf := &bytes.Buffer{}
		log := New(Config{Level: "warn", Format: "json", Output: buf})

		log.Info("info msg")
		assert.Empty(t, buf.String(), "info should not be logged at warn level")

		log.Warn("warn msg")
		assert.Contains(t, buf.String(), "warn msg")
	})

	t.Run("error_level", func(t *testing.T) {
		buf := &bytes.Buffer{}
		log := New(Config{Level: "error", Format: "json", Output: buf})

		log.Warn("warn msg")
		assert.Empty(t, buf.String(), "warn should not be logged at error level")

		log.Error("error msg")
		assert.Contains(t, buf.String(), "error msg")
	})

	t.Run("default_level_is_info", func(t *testing.T) {
		buf := &bytes.Buffer{}
		log := New(Config{Level: "unknown", Format: "json", Output: buf})

		log.Debug("debug msg")
		assert.Empty(t, buf.String(), "debug should not be logged at default info level")

		log.Info("info msg")
		assert.Contains(t, buf.String(), "info msg")
	})
}

func TestDefault(t *testing.T) {
	log := Default()
	require.NotNil(t, log)
	require.NotNil(t, log.Logger)
}

func TestWithContext(t *testing.T) {
	t.Run("with_request_id", func(t *testing.T) {
		buf := &bytes.Buffer{}
		log := New(Config{Level: "info", Format: "json", Output: buf})

		ctx := context.WithValue(context.Background(), RequestIDKey, "req-123")
		ctxLog := log.WithContext(ctx)

		ctxLog.Info("with context")
		assert.Contains(t, buf.String(), "req-123")
	})

	t.Run("with_user_id", func(t *testing.T) {
		buf := &bytes.Buffer{}
		log := New(Config{Level: "info", Format: "json", Output: buf})

		ctx := context.WithValue(context.Background(), UserIDKey, "user-456")
		ctxLog := log.WithContext(ctx)

		ctxLog.Info("with user")
		assert.Contains(t, buf.String(), "user-456")
	})

	t.Run("with_trace_id", func(t *testing.T) {
		buf := &bytes.Buffer{}
		log := New(Config{Level: "info", Format: "json", Output: buf})

		ctx := context.WithValue(context.Background(), TraceIDKey, "trace-789")
		ctxLog := log.WithContext(ctx)

		ctxLog.Info("with trace")
		assert.Contains(t, buf.String(), "trace-789")
	})

	t.Run("with_ip_address", func(t *testing.T) {
		buf := &bytes.Buffer{}
		log := New(Config{Level: "info", Format: "json", Output: buf})

		ctx := context.WithValue(context.Background(), IPAddressKey, "203.0.113.42")
		ctxLog := log.WithContext(ctx)

		ctxLog.Info("with ip")
		assert.Contains(t, buf.String(), "203.0.113.42")
		assert.Contains(t, buf.String(), "ip_address")
	})

	t.Run("with_user_agent", func(t *testing.T) {
		buf := &bytes.Buffer{}
		log := New(Config{Level: "info", Format: "json", Output: buf})

		ctx := context.WithValue(context.Background(), UserAgentKey, "Mozilla/5.0 (test)")
		ctxLog := log.WithContext(ctx)

		ctxLog.Info("with ua")
		assert.Contains(t, buf.String(), "Mozilla/5.0")
		assert.Contains(t, buf.String(), "user_agent")
	})

	t.Run("empty_context_returns_same_logger", func(t *testing.T) {
		log := New(Config{Level: "info", Format: "json", Output: &bytes.Buffer{}})
		ctxLog := log.WithContext(context.Background())
		assert.Equal(t, log, ctxLog)
	})
}

func TestWithField(t *testing.T) {
	buf := &bytes.Buffer{}
	log := New(Config{Level: "info", Format: "json", Output: buf})

	fieldLog := log.WithField("component", "worker")
	fieldLog.Info("test")

	assert.Contains(t, buf.String(), "component")
	assert.Contains(t, buf.String(), "worker")
}

func TestWithFields(t *testing.T) {
	buf := &bytes.Buffer{}
	log := New(Config{Level: "info", Format: "json", Output: buf})

	fieldLog := log.WithFields(map[string]any{
		"service": "api",
		"version": "1.0",
	})
	fieldLog.Info("test")

	assert.Contains(t, buf.String(), "service")
	assert.Contains(t, buf.String(), "api")
	assert.Contains(t, buf.String(), "version")
}

func TestWithError(t *testing.T) {
	buf := &bytes.Buffer{}
	log := New(Config{Level: "info", Format: "json", Output: buf})

	errLog := log.WithError(errors.New("something failed"))
	errLog.Info("handling error")

	assert.Contains(t, buf.String(), "error")
	assert.Contains(t, buf.String(), "something failed")
}
