package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"time"
)

// ContextKey type for context values
type ContextKey string

const (
	// RequestIDKey is the context key for request ID
	RequestIDKey ContextKey = "request_id"
	// UserIDKey is the context key for user ID
	UserIDKey ContextKey = "user_id"
	// TraceIDKey is the context key for trace ID
	TraceIDKey ContextKey = "trace_id"
)

// Logger wraps slog.Logger with additional functionality
type Logger struct {
	*slog.Logger
}

// Config holds logger configuration
type Config struct {
	Level  string // "debug", "info", "warn", "error"
	Format string // "json", "text"
	Output io.Writer
}

// New creates a new Logger instance
func New(cfg Config) *Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Format time as ISO 8601 for Loki compatibility
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format(time.RFC3339Nano))
				}
			}
			// Shorten source path
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					a.Value = slog.StringValue(source.File + ":" + itoa(source.Line))
				}
			}
			return a
		},
	}

	var handler slog.Handler
	if cfg.Format == "text" {
		handler = slog.NewTextHandler(output, opts)
	} else {
		// Default to JSON for Loki compatibility
		handler = slog.NewJSONHandler(output, opts)
	}

	return &Logger{
		Logger: slog.New(handler),
	}
}

// Default creates a logger with default configuration (JSON, Info level)
func Default() *Logger {
	return New(Config{
		Level:  "info",
		Format: "json",
		Output: os.Stdout,
	})
}

// WithContext returns a new logger with context values as attributes
func (l *Logger) WithContext(ctx context.Context) *Logger {
	attrs := []any{}

	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		attrs = append(attrs, "request_id", requestID)
	}
	if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
		attrs = append(attrs, "user_id", userID)
	}
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		attrs = append(attrs, "trace_id", traceID)
	}

	if len(attrs) == 0 {
		return l
	}

	return &Logger{
		Logger: l.Logger.With(attrs...),
	}
}

// WithField returns a new logger with the given field
func (l *Logger) WithField(key string, value any) *Logger {
	return &Logger{
		Logger: l.Logger.With(key, value),
	}
}

// WithFields returns a new logger with the given fields
func (l *Logger) WithFields(fields map[string]any) *Logger {
	attrs := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	return &Logger{
		Logger: l.Logger.With(attrs...),
	}
}

// WithError returns a new logger with the error as a field
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		Logger: l.Logger.With("error", err.Error()),
	}
}

// WithCaller returns a new logger with caller information
func (l *Logger) WithCaller() *Logger {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		return l
	}
	return &Logger{
		Logger: l.Logger.With("caller", file+":"+itoa(line)),
	}
}

// itoa converts int to string without importing strconv
func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}
