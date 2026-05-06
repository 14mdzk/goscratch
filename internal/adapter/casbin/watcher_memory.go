package casbin

import (
	"context"
	"log/slog"

	"github.com/casbin/casbin/v3/model"
)

const memoryWatcherBufSize = 64

// MemoryWatcher is an in-process Casbin WatcherEx that delivers policy-change
// signals via an internal buffered channel.  It is designed for tests and
// single-binary deployments where all Casbin enforcers share the same process.
// Call Start to launch the dispatch goroutine before using the watcher.
type MemoryWatcher struct {
	ch       chan string
	callback func(string)
}

// NewMemoryWatcher creates a MemoryWatcher with a buffered channel.
// Call Start(ctx) before use.
func NewMemoryWatcher() *MemoryWatcher {
	return &MemoryWatcher{
		ch: make(chan string, memoryWatcherBufSize),
	}
}

// Start launches the background goroutine that reads the channel and invokes
// the registered callback.  It exits when ctx is cancelled or the channel is
// closed.
func (w *MemoryWatcher) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-w.ch:
				if !ok {
					return
				}
				if w.callback != nil {
					w.callback(msg)
				} else {
					slog.Warn("MemoryWatcher: no callback registered, dropping message", "msg", msg)
				}
			}
		}
	}()
}

// SetUpdateCallback stores the callback that will be invoked for each message.
func (w *MemoryWatcher) SetUpdateCallback(f func(string)) error {
	w.callback = f
	return nil
}

// Update sends a full-reload signal.
func (w *MemoryWatcher) Update() error {
	return w.send(encodeOp("reload", "", "", nil))
}

// UpdateForAddPolicy sends an incremental add-policy signal.
func (w *MemoryWatcher) UpdateForAddPolicy(sec, ptype string, params ...string) error {
	return w.send(encodeOp("add_policy", sec, ptype, params))
}

// UpdateForRemovePolicy sends an incremental remove-policy signal.
func (w *MemoryWatcher) UpdateForRemovePolicy(sec, ptype string, params ...string) error {
	return w.send(encodeOp("remove_policy", sec, ptype, params))
}

// UpdateForRemoveFilteredPolicy sends a full-reload signal (safe fallback for
// filtered removes whose result set is non-trivial to replicate incrementally).
func (w *MemoryWatcher) UpdateForRemoveFilteredPolicy(sec, ptype string, fieldIndex int, fieldValues ...string) error {
	return w.send(encodeOp("reload", "", "", nil))
}

// UpdateForSavePolicy sends a full-reload signal.
func (w *MemoryWatcher) UpdateForSavePolicy(_ model.Model) error {
	return w.send(encodeOp("reload", "", "", nil))
}

// UpdateForAddPolicies sends one incremental add-policy signal per rule.
func (w *MemoryWatcher) UpdateForAddPolicies(sec, ptype string, rules ...[]string) error {
	for _, rule := range rules {
		if err := w.send(encodeOp("add_policy", sec, ptype, rule)); err != nil {
			return err
		}
	}
	return nil
}

// UpdateForRemovePolicies sends one incremental remove-policy signal per rule.
func (w *MemoryWatcher) UpdateForRemovePolicies(sec, ptype string, rules ...[]string) error {
	for _, rule := range rules {
		if err := w.send(encodeOp("remove_policy", sec, ptype, rule)); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the internal channel, which causes the dispatch goroutine to exit.
func (w *MemoryWatcher) Close() {
	close(w.ch)
}

// send is a non-blocking helper that drops messages when the buffer is full.
func (w *MemoryWatcher) send(msg string) error {
	select {
	case w.ch <- msg:
	default:
		slog.Warn("MemoryWatcher: channel full, dropping message", "msg", msg)
	}
	return nil
}
