package config

import (
	"fmt"
	"strings"
)

// QueueBackend names the queue tier a deployment runs on.
//
//   - QueueBackendGoroutine is the default: zero infrastructure, jobs live
//     only in process memory and die with the process (best-effort
//     durability, documented in docs/features/background-jobs.md).
//   - QueueBackendRabbitMQ is the durable tier: needs a reachable broker.
//   - QueueBackendRedis is reserved for a future tier and fails boot.
type QueueBackend string

const (
	QueueBackendGoroutine QueueBackend = "goroutine"
	QueueBackendRedis     QueueBackend = "redis"
	QueueBackendRabbitMQ  QueueBackend = "rabbitmq"
)

// QueueConfig selects the queue backend.
//
// Backend takes one of goroutine, redis, rabbitmq. Empty means unset: the
// deprecated rabbitmq.enabled alias applies instead (rabbitmq when true,
// goroutine when false). Unknown values fail boot.
type QueueConfig struct {
	Backend string `json:"backend" env:"QUEUE_BACKEND"`
}

// ResolveQueueBackend maps raw configuration to the effective backend.
//
// Resolution order: explicit queue.backend wins; otherwise the deprecated
// rabbitmq.enabled alias decides (true -> rabbitmq, false -> goroutine).
// The reserved redis value and unknown values are boot errors, never
// silent fallbacks: a deployment must say what it runs on.
func (c *Config) ResolveQueueBackend() (QueueBackend, error) {
	raw := QueueBackend(strings.ToLower(strings.TrimSpace(c.Queue.Backend)))
	if raw == "" {
		if c.RabbitMQ.Enabled {
			return QueueBackendRabbitMQ, nil
		}
		return QueueBackendGoroutine, nil
	}
	switch raw {
	case QueueBackendGoroutine:
		return QueueBackendGoroutine, nil
	case QueueBackendRabbitMQ:
		return QueueBackendRabbitMQ, nil
	case QueueBackendRedis:
		return "", fmt.Errorf("queue.backend %q is reserved for a future Redis tier and is not implemented in v1.3", c.Queue.Backend)
	default:
		return "", fmt.Errorf("unknown queue.backend %q: want one of goroutine, redis, rabbitmq", c.Queue.Backend)
	}
}

// ValidateQueueWorker enforces the backend/in-process matrix: the goroutine
// tier is only consumed in-process, and the RabbitMQ tier never shares
// consumption with the API process (run cmd/worker instead). Contradictory
// combinations fail boot with the corrective setting named.
func (c *Config) ValidateQueueWorker(backend QueueBackend) error {
	switch backend {
	case QueueBackendGoroutine:
		if !c.Worker.InProcess {
			return fmt.Errorf("queue.backend=goroutine requires worker.in_process=true: the in-memory tier is only consumed in-process (set WORKER_IN_PROCESS=true or switch queue.backend)")
		}
	case QueueBackendRabbitMQ:
		if c.Worker.InProcess {
			return fmt.Errorf("queue.backend=rabbitmq forbids worker.in_process=true: the API must not join consumption as a competing consumer (run cmd/worker instead)")
		}
	case QueueBackendRedis:
		return fmt.Errorf("queue.backend=redis is reserved and not implemented in v1.3")
	default:
		return fmt.Errorf("unknown queue backend %q", backend)
	}
	return nil
}
