package usecase

import "context"

// Revoker exposes session-revocation operations to other modules (e.g. user)
// without leaking auth internals. The concrete implementation lives in
// authUseCase and knows the dual-key cache shape.
type Revoker interface {
	// RevokeAllForUser deletes every active refresh token for the given userID.
	// It is called by ChangePassword to terminate all existing sessions.
	RevokeAllForUser(ctx context.Context, userID string) error
}
