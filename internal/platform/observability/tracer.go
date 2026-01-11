package observability

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

// TracerConfig holds tracing configuration
type TracerConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	Endpoint       string // OTLP endpoint (e.g., "localhost:4318")
	Enabled        bool
}

// InitTracer initializes the OpenTelemetry tracer
func InitTracer(ctx context.Context, cfg TracerConfig) (func(context.Context) error, error) {
	if !cfg.Enabled {
		// Return no-op shutdown function
		tracer = otel.Tracer(cfg.ServiceName)
		return func(context.Context) error { return nil }, nil
	}

	// Create OTLP exporter
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.Endpoint),
		otlptracehttp.WithInsecure(), // Use WithTLSCredentials for production
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service info
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Get tracer
	tracer = tp.Tracer(cfg.ServiceName)

	// Return shutdown function
	return tp.Shutdown, nil
}

// GetTracer returns the global tracer
func GetTracer() trace.Tracer {
	if tracer == nil {
		return otel.Tracer("goscratch")
	}
	return tracer
}

// TracingMiddleware returns a Fiber middleware that adds OpenTelemetry tracing
func TracingMiddleware(serviceName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract trace context from incoming request
		ctx := c.UserContext()

		// Start a new span
		tr := GetTracer()
		ctx, span := tr.Start(ctx, fmt.Sprintf("%s %s", c.Method(), c.Route().Path),
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPMethod(c.Method()),
				semconv.HTTPRoute(c.Route().Path),
				semconv.HTTPURL(c.OriginalURL()),
				attribute.String("http.user_agent", c.Get("User-Agent")),
				semconv.NetHostName(c.Hostname()),
			),
		)
		defer span.End()

		// Store span context in Fiber context
		c.SetUserContext(ctx)

		// Add trace ID to response header for debugging
		traceID := span.SpanContext().TraceID().String()
		c.Set("X-Trace-ID", traceID)

		// Process request
		err := c.Next()

		// Add response attributes
		span.SetAttributes(
			semconv.HTTPStatusCode(c.Response().StatusCode()),
		)

		// Record error if present
		if err != nil {
			span.RecordError(err)
		}

		return err
	}
}

// StartSpan starts a new span with the given name
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return GetTracer().Start(ctx, name, opts...)
}

// SpanFromContext returns the current span from context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddSpanAttributes adds attributes to the current span
func AddSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// RecordSpanError records an error on the current span
func RecordSpanError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
}

// WrapDBOperation wraps a database operation with tracing
func WrapDBOperation(ctx context.Context, operation, table string) (context.Context, trace.Span) {
	return StartSpan(ctx, fmt.Sprintf("db.%s", operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.operation", operation),
			attribute.String("db.table", table),
			attribute.String("db.system", "postgresql"),
		),
	)
}

// WrapCacheOperation wraps a cache operation with tracing
func WrapCacheOperation(ctx context.Context, operation, key string) (context.Context, trace.Span) {
	return StartSpan(ctx, fmt.Sprintf("cache.%s", operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("cache.operation", operation),
			attribute.String("cache.key", key),
			attribute.String("cache.system", "redis"),
		),
	)
}
