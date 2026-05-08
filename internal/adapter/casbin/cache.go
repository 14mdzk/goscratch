package casbin

import (
	"container/list"
	"strings"
	"sync"
)

// decisionCache is a thread-safe LRU cache mapping "sub\x00obj\x00act" keys to
// bool authorization decisions. A maxSize of 0 disables the cache entirely;
// all operations become no-ops and lookups always return a miss.
//
// Thread safety: a single sync.Mutex guards both the map and the list.
// Get takes a write-lock because LRU promotion mutates list order.
type decisionCache struct {
	mu      sync.Mutex
	maxSize int
	items   map[string]*list.Element
	order   *list.List
}

type cacheEntry struct {
	key   string
	value bool
}

// newDecisionCache creates a decision cache with the given maximum number of
// entries. Pass 0 to disable caching (all lookups miss, puts are no-ops).
func newDecisionCache(maxSize int) *decisionCache {
	return &decisionCache{
		maxSize: maxSize,
		items:   make(map[string]*list.Element, maxSize),
		order:   list.New(),
	}
}

// cacheKey encodes (sub, obj, act) into a single map key using a \x00
// separator. Callers (get/put) must reject inputs that contain \x00 before
// calling this function; see the null-byte guards in get and put.
func cacheKey(sub, obj, act string) string {
	// pre-allocate: len(sub)+1+len(obj)+1+len(act)
	b := make([]byte, 0, len(sub)+1+len(obj)+1+len(act))
	b = append(b, sub...)
	b = append(b, '\x00')
	b = append(b, obj...)
	b = append(b, '\x00')
	b = append(b, act...)
	return string(b)
}

// get looks up a decision. Returns (value, true) on a cache hit.
// On a hit the entry is promoted to MRU position.
// A nil receiver is treated as a disabled cache (always misses).
// If any argument contains \x00 (the cache key separator), the lookup is
// skipped to prevent cache key collisions from untrusted Enforce input.
func (c *decisionCache) get(sub, obj, act string) (value, ok bool) {
	if c == nil || c.maxSize == 0 {
		return false, false
	}
	if strings.ContainsRune(sub, 0) || strings.ContainsRune(obj, 0) || strings.ContainsRune(act, 0) {
		return false, false
	}
	key := cacheKey(sub, obj, act)
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return false, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*cacheEntry).value, true
}

// put stores a decision. If the cache is full the LRU (tail) entry is evicted.
// A nil receiver is a no-op.
// If any argument contains \x00 (the cache key separator), the put is
// skipped to prevent cache key collisions from untrusted Enforce input.
func (c *decisionCache) put(sub, obj, act string, value bool) {
	if c == nil || c.maxSize == 0 {
		return
	}
	if strings.ContainsRune(sub, 0) || strings.ContainsRune(obj, 0) || strings.ContainsRune(act, 0) {
		return
	}
	key := cacheKey(sub, obj, act)
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		el.Value.(*cacheEntry).value = value
		c.order.MoveToFront(el)
		return
	}
	if c.order.Len() >= c.maxSize {
		c.evictLRU()
	}
	entry := &cacheEntry{key: key, value: value}
	el := c.order.PushFront(entry)
	c.items[key] = el
}

// evictLRU removes the least-recently-used entry. Must be called with mu held.
func (c *decisionCache) evictLRU() {
	tail := c.order.Back()
	if tail == nil {
		return
	}
	c.order.Remove(tail)
	delete(c.items, tail.Value.(*cacheEntry).key)
}

// invalidateSub removes all entries where the subject field equals sub.
// A nil receiver is a no-op.
func (c *decisionCache) invalidateSub(sub string) {
	if c == nil || c.maxSize == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	prefix := sub + "\x00"
	for key, el := range c.items {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			c.order.Remove(el)
			delete(c.items, key)
		}
	}
}

// flush removes all entries from the cache.
// A nil receiver is a no-op.
func (c *decisionCache) flush() {
	if c == nil || c.maxSize == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element, c.maxSize)
	c.order.Init()
}

// len returns the current number of cached entries (for testing).
// A nil receiver returns 0.
func (c *decisionCache) len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
