package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value   V
	expires time.Time
}
type Cache[K comparable, V any] interface {
	Get(K) (V, bool)
	Set(K, V, time.Duration)
	Delete(K)
}
type TTL[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]entry[V]
}

func New[K comparable, V any](interval time.Duration) *TTL[K, V] {
	c := &TTL[K, V]{items: make(map[K]entry[V])}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for range t.C {
			c.mu.Lock()
			for k, v := range c.items {
				if time.Now().After(v.expires) {
					delete(c.items, k)
				}
			}
			c.mu.Unlock()
		}
	}()
	return c
}
func (c *TTL[K, V]) Get(k K) (V, bool) {
	c.mu.RLock()
	v, ok := c.items[k]
	c.mu.RUnlock()
	if !ok || time.Now().After(v.expires) {
		var zero V
		return zero, false
	}
	return v.value, true
}
func (c *TTL[K, V]) Set(k K, v V, ttl time.Duration) {
	c.mu.Lock()
	c.items[k] = entry[V]{v, time.Now().Add(ttl)}
	c.mu.Unlock()
}
func (c *TTL[K, V]) Delete(k K) { c.mu.Lock(); delete(c.items, k); c.mu.Unlock() }
