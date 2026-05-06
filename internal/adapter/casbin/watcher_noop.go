package casbin

import "github.com/casbin/casbin/v3/model"

// NoopWatcher is a no-operation Casbin WatcherEx.
// It satisfies the persist.WatcherEx interface without performing any I/O.
// It is useful in tests and single-instance deployments that rely solely
// on the backstop tick for policy synchronisation.
type NoopWatcher struct {
	callback func(string)
}

// NewNoopWatcher creates a new NoopWatcher.
func NewNoopWatcher() *NoopWatcher {
	return &NoopWatcher{}
}

// SetUpdateCallback stores the callback.
func (w *NoopWatcher) SetUpdateCallback(f func(string)) error {
	w.callback = f
	return nil
}

// Update is a no-op.
func (w *NoopWatcher) Update() error { return nil }

// Close is a no-op.
func (w *NoopWatcher) Close() {}

// UpdateForAddPolicy is a no-op.
func (w *NoopWatcher) UpdateForAddPolicy(sec, ptype string, params ...string) error { return nil }

// UpdateForRemovePolicy is a no-op.
func (w *NoopWatcher) UpdateForRemovePolicy(sec, ptype string, params ...string) error { return nil }

// UpdateForRemoveFilteredPolicy is a no-op.
func (w *NoopWatcher) UpdateForRemoveFilteredPolicy(sec, ptype string, fieldIndex int, fieldValues ...string) error {
	return nil
}

// UpdateForSavePolicy is a no-op.
func (w *NoopWatcher) UpdateForSavePolicy(m model.Model) error { return nil }

// UpdateForAddPolicies is a no-op.
func (w *NoopWatcher) UpdateForAddPolicies(sec, ptype string, rules ...[]string) error {
	return nil
}

// UpdateForRemovePolicies is a no-op.
func (w *NoopWatcher) UpdateForRemovePolicies(sec, ptype string, rules ...[]string) error {
	return nil
}
