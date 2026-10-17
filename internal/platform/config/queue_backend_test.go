package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveQueueBackend(t *testing.T) {
	tests := []struct {
		name      string
		backend   string
		enabled   bool
		want      QueueBackend
		wantError string
	}{
		{name: "unset and rabbitmq disabled resolves goroutine", backend: "", enabled: false, want: QueueBackendGoroutine},
		{name: "unset and rabbitmq enabled resolves rabbitmq via alias", backend: "", enabled: true, want: QueueBackendRabbitMQ},
		{name: "explicit goroutine wins over enabled alias", backend: "goroutine", enabled: true, want: QueueBackendGoroutine},
		{name: "explicit rabbitmq", backend: "rabbitmq", enabled: false, want: QueueBackendRabbitMQ},
		{name: "values are case-insensitive", backend: "RabbitMQ", enabled: false, want: QueueBackendRabbitMQ},
		{name: "surrounding whitespace is trimmed", backend: "  goroutine\n", enabled: false, want: QueueBackendGoroutine},
		{name: "redis is reserved", backend: "redis", enabled: false, wantError: "reserved"},
		{name: "unknown values fail boot", backend: "kafka", enabled: false, wantError: "unknown queue.backend"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Queue: QueueConfig{Backend: tt.backend}, RabbitMQ: RabbitMQConfig{Enabled: tt.enabled}}
			got, err := cfg.ResolveQueueBackend()
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateQueueWorker(t *testing.T) {
	tests := []struct {
		name      string
		backend   QueueBackend
		inProcess bool
		wantError string
	}{
		{name: "goroutine requires in-process", backend: QueueBackendGoroutine, inProcess: true},
		{name: "goroutine without in-process fails boot", backend: QueueBackendGoroutine, inProcess: false, wantError: "worker.in_process=true"},
		{name: "rabbitmq forbids in-process", backend: QueueBackendRabbitMQ, inProcess: true, wantError: "competing consumer"},
		{name: "rabbitmq with external worker", backend: QueueBackendRabbitMQ, inProcess: false},
		{name: "redis is refused", backend: QueueBackendRedis, inProcess: true, wantError: "reserved"},
		{name: "unknown backend is refused", backend: QueueBackend("kafka"), inProcess: false, wantError: "unknown queue backend"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Worker: WorkerConfig{InProcess: tt.inProcess}}
			err := cfg.ValidateQueueWorker(tt.backend)
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
				return
			}
			assert.NoError(t, err)
		})
	}
}
