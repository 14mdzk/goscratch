package health

import (
	"context"
	"errors"

	"github.com/14mdzk/goscratch/internal/adapter/cache"
	casbinadapter "github.com/14mdzk/goscratch/internal/adapter/casbin"
	"github.com/14mdzk/goscratch/internal/adapter/queue"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthChecker is the contract for a single dependency health check.
// Name returns a short human-readable identifier used in readiness responses.
// Check returns nil when the dependency is healthy; a non-nil error with a
// short, non-sensitive reason when it is not.
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) error
}

// postgresChecker pings the Postgres connection pool.
type postgresChecker struct {
	pool *pgxpool.Pool
}

// NewPostgresChecker returns a HealthChecker that pings the pgx pool.
func NewPostgresChecker(pool *pgxpool.Pool) HealthChecker {
	return &postgresChecker{pool: pool}
}

func (c *postgresChecker) Name() string { return "database" }

func (c *postgresChecker) Check(ctx context.Context) error {
	if err := c.pool.Ping(ctx); err != nil {
		return errors.New("ping failed")
	}
	return nil
}

// cacheChecker pings the Redis cache.
// Reports "cache(noop)" and returns nil when the adapter is a *cache.NoOpCache
// so it does not fail readiness on intentionally-disabled cache deployments.
type cacheChecker struct {
	c port.Cache
}

// NewCacheChecker returns a HealthChecker for the cache adapter.
func NewCacheChecker(c port.Cache) HealthChecker {
	return &cacheChecker{c: c}
}

func (c *cacheChecker) Name() string {
	if _, ok := c.c.(*cache.NoOpCache); ok {
		return "cache(noop)"
	}
	return "cache"
}

func (c *cacheChecker) Check(ctx context.Context) error {
	if _, ok := c.c.(*cache.NoOpCache); ok {
		return nil
	}
	// Exists issues a single-RTT EXISTS command without reading or writing data.
	_, err := c.c.Exists(ctx, "__healthz_probe__")
	if err != nil {
		return errors.New("ping failed")
	}
	return nil
}

// queueChecker checks the RabbitMQ connection via port.Queue.Ping.
// Reports "queue(noop)" and returns nil when the adapter is a *queue.NoOpQueue.
// Ping issues a passive AMQP declare on a transient channel and never creates
// broker-side resources, so a fresh broker does not accumulate a sentinel queue.
type queueChecker struct {
	q port.Queue
}

// NewQueueChecker returns a HealthChecker for the queue adapter.
func NewQueueChecker(q port.Queue) HealthChecker {
	return &queueChecker{q: q}
}

func (c *queueChecker) Name() string {
	if _, ok := c.q.(*queue.NoOpQueue); ok {
		return "queue(noop)"
	}
	return "queue"
}

func (c *queueChecker) Check(ctx context.Context) error {
	if _, ok := c.q.(*queue.NoOpQueue); ok {
		return nil
	}
	if err := c.q.Ping(ctx); err != nil {
		return errors.New("connection unavailable")
	}
	return nil
}

// authzChecker verifies that the Casbin authorizer is live.
// Reports "authz(noop)" and returns nil when the adapter is *casbinadapter.NoOpAdapter.
type authzChecker struct {
	a port.Authorizer
}

// NewAuthzChecker returns a HealthChecker for the authorizer.
func NewAuthzChecker(a port.Authorizer) HealthChecker {
	return &authzChecker{a: a}
}

func (c *authzChecker) Name() string {
	if _, ok := c.a.(*casbinadapter.NoOpAdapter); ok {
		return "authz(noop)"
	}
	return "authz"
}

func (c *authzChecker) Check(ctx context.Context) error {
	if _, ok := c.a.(*casbinadapter.NoOpAdapter); ok {
		return nil
	}
	// Enforce a probe subject/object/action that cannot exist in any real policy.
	// The call exercises the enforcer and its DB handle; we ignore the boolean
	// result and only care that no connection-level error is returned.
	_, err := c.a.EnforceWithContext(ctx, "__healthz__", "__probe__", "__ping__")
	if err != nil {
		return errors.New("enforce failed")
	}
	return nil
}
