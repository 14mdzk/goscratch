package casbin

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

// TestRedisWatcherDefaultChannelIsVersioned exercises the empty-channel contract
// of NewRedisWatcher: passing "" must resolve to the versioned default
// `casbin:policy:update:v1`. The `:vN` suffix isolates subscribers by
// message-envelope version so a future protocol bump can ship behind a
// `:v2` rename and old/new instances on a shared Redis cannot misparse each
// other's payloads during a rolling deploy. Changing the literal without
// bumping the suffix breaks that isolation.
func TestRedisWatcherDefaultChannelIsVersioned(t *testing.T) {
	const want = "casbin:policy:update:v1"

	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = client.Close() })

	w, err := NewRedisWatcher(context.Background(), client, "")
	if err != nil {
		t.Fatalf("NewRedisWatcher returned error: %v", err)
	}
	t.Cleanup(w.Close)

	if w.channel != want {
		t.Fatalf("default channel = %q, want %q (bump the :vN suffix when the message envelope shape changes)", w.channel, want)
	}
}
