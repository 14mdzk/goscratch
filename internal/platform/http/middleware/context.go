package middleware

import (
	"context"

	"github.com/14mdzk/goscratch/pkg/logger"
)

// setContextValue is a helper to set a value in context
func setContextValue(ctx context.Context, key logger.ContextKey, value string) context.Context {
	return context.WithValue(ctx, key, value)
}
