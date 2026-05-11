package casbin

import (
	"context"
	"log/slog"
	"sync"

	"github.com/casbin/casbin/v3/model"
	"github.com/redis/go-redis/v9"
)

// defaultRedisChannel carries an explicit `:v1` suffix so that any future
// change to the pub/sub message envelope can be shipped behind a `:v2` bump.
// Old and new instances on a shared Redis then subscribe to disjoint channels
// during a rolling deploy and cannot misparse each other's payloads. The
// back-stop full reload tick still converges all instances to the database
// state regardless of channel skew.
const defaultRedisChannel = "casbin:policy:update:v1"

// RedisWatcher is a Casbin WatcherEx that distributes policy-change signals
// across multiple instances via Redis Pub/Sub.  All instances that share the
// same Redis channel will have their registered callback invoked whenever any
// instance mutates the policy.
type RedisWatcher struct {
	client   *redis.Client
	channel  string
	pubsub   *redis.PubSub
	callback func(string)
	mu       sync.RWMutex
}

// NewRedisWatcher creates a RedisWatcher and starts the subscriber goroutine.
// channel defaults to "casbin:policy:update:v1" when empty.
func NewRedisWatcher(ctx context.Context, client *redis.Client, channel string) (*RedisWatcher, error) {
	if channel == "" {
		channel = defaultRedisChannel
	}
	w := &RedisWatcher{
		client:  client,
		channel: channel,
	}
	w.pubsub = client.Subscribe(ctx, channel)
	go w.listen(ctx)
	return w, nil
}

// SetUpdateCallback stores the callback under a write lock.
func (w *RedisWatcher) SetUpdateCallback(f func(string)) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.callback = f
	return nil
}

// Update publishes a full-reload signal to all subscribers.
func (w *RedisWatcher) Update() error {
	return w.publish(encodeOp("reload", "", "", nil))
}

// UpdateForAddPolicy publishes an incremental add-policy signal.
func (w *RedisWatcher) UpdateForAddPolicy(sec, ptype string, params ...string) error {
	return w.publish(encodeOp("add_policy", sec, ptype, params))
}

// UpdateForRemovePolicy publishes an incremental remove-policy signal.
func (w *RedisWatcher) UpdateForRemovePolicy(sec, ptype string, params ...string) error {
	return w.publish(encodeOp("remove_policy", sec, ptype, params))
}

// UpdateForRemoveFilteredPolicy publishes a full-reload signal.
func (w *RedisWatcher) UpdateForRemoveFilteredPolicy(sec, ptype string, fieldIndex int, fieldValues ...string) error {
	return w.publish(encodeOp("reload", "", "", nil))
}

// UpdateForSavePolicy publishes a full-reload signal.
func (w *RedisWatcher) UpdateForSavePolicy(_ model.Model) error {
	return w.publish(encodeOp("reload", "", "", nil))
}

// UpdateForAddPolicies publishes one incremental signal per rule.
func (w *RedisWatcher) UpdateForAddPolicies(sec, ptype string, rules ...[]string) error {
	for _, rule := range rules {
		if err := w.publish(encodeOp("add_policy", sec, ptype, rule)); err != nil {
			return err
		}
	}
	return nil
}

// UpdateForRemovePolicies publishes one incremental signal per rule.
func (w *RedisWatcher) UpdateForRemovePolicies(sec, ptype string, rules ...[]string) error {
	for _, rule := range rules {
		if err := w.publish(encodeOp("remove_policy", sec, ptype, rule)); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the Pub/Sub subscription.
func (w *RedisWatcher) Close() {
	if err := w.pubsub.Close(); err != nil {
		slog.Warn("RedisWatcher: error closing pubsub", "error", err)
	}
}

// publish sends a message to all subscribers on the configured channel.
func (w *RedisWatcher) publish(msg string) error {
	return w.client.Publish(context.Background(), w.channel, msg).Err()
}

// listen reads messages from the Pub/Sub channel and invokes the callback.
func (w *RedisWatcher) listen(ctx context.Context) {
	ch := w.pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			w.mu.RLock()
			cb := w.callback
			w.mu.RUnlock()
			if cb != nil {
				cb(msg.Payload)
			}
		}
	}
}
