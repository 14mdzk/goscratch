package dto

// LoginRequest represents the login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents the login response.
// UserID is populated by the usecase but excluded from JSON output; the audit
// decorator consumes it to populate AuditEntry.ResourceID without re-parsing
// the access token.
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds
	TokenType    string `json:"token_type"`
	UserID       string `json:"-"`
}

// RefreshRequest represents the token refresh request.
// Only the opaque refresh token is required; the server resolves the user ID
// from the lookup key and does not accept a client-supplied user_id hint.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// RefreshResponse represents the token refresh response
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// LogoutRequest represents the logout request.
// The caller ID is populated from the JWT claims by the handler, not from the
// request body — the handler extracts it after the Auth middleware runs.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}
