package domain

import "time"

// Claims is the domain representation of an authenticated caller's identity.
// Handlers and usecases depend on this type; they must not import JWT library
// types. The JWT layer in internal/platform/http/middleware maps between
// jwt.RegisteredClaims and this struct.
type Claims struct {
	// Subject is the standard JWT "sub" claim — the user's ID.
	Subject string
	UserID  string
	Email   string
	Name    string

	// Token validity fields.
	Issuer    string
	Audience  []string
	ExpiresAt time.Time
	IssuedAt  time.Time
	NotBefore time.Time
}
