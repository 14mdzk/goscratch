package port

import "context"

// Transactor executes a callback inside an atomic database transaction.
// Repositories participate automatically by reading the active transaction
// from ctx, keeping usecases decoupled from the concrete database driver.
type Transactor interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
