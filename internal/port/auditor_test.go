package port_test

import (
	"context"
	"testing"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestExtractAuditContext_TypedKeys(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, logger.UserIDKey, "u-1")
	ctx = context.WithValue(ctx, logger.IPAddressKey, "203.0.113.42")
	ctx = context.WithValue(ctx, logger.UserAgentKey, "Mozilla/5.0 (test)")

	ac := port.ExtractAuditContext(ctx)

	assert.Equal(t, "u-1", ac.UserID)
	assert.Equal(t, "203.0.113.42", ac.IPAddress)
	assert.Equal(t, "Mozilla/5.0 (test)", ac.UserAgent)
}

// TestExtractAuditContext_StringLiteralKeys_DoesNotPopulate locks the bug
// from regressing: prior to PR #1 the auditor read with bare string keys
// while writers used typed logger.ContextKey values, producing empty audit
// rows. If a future change reintroduces ctx.Value("user_id") this test will
// fail.
func TestExtractAuditContext_StringLiteralKeys_DoesNotPopulate(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "user_id", "u-1")        //nolint:staticcheck // intentional negative test
	ctx = context.WithValue(ctx, "ip_address", "1.2.3.4") //nolint:staticcheck // intentional negative test
	ctx = context.WithValue(ctx, "user_agent", "ua")      //nolint:staticcheck // intentional negative test

	ac := port.ExtractAuditContext(ctx)

	assert.Empty(t, ac.UserID)
	assert.Empty(t, ac.IPAddress)
	assert.Empty(t, ac.UserAgent)
}

func TestExtractAuditContext_EmptyContext(t *testing.T) {
	ac := port.ExtractAuditContext(context.Background())
	assert.Empty(t, ac.UserID)
	assert.Empty(t, ac.IPAddress)
	assert.Empty(t, ac.UserAgent)
}
