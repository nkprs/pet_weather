package cache

import (
	"sync"
	"time"
)

type cacheItem[T any] struct {
	val       T
	expiredAt time.Time
}

type Cache[T any] struct {
	mu    sync.RWMutex
	items map[string]cacheItem[T]
	ttl   time.Duration
}

func New[T any](ttl time.Duration) *Cache[T] {
	return &Cache[T]{
		items: make(map[string]cacheItem[T]),
		ttl:   ttl,
	}
}

func (c *Cache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()

	var zero T
	if !ok || time.Now().After(it.expiredAt) {
		return zero, false
	}
	return it.val, true
}

func (c *Cache[T]) Set(key string, val T) {
	c.mu.Lock()
	c.items[key] = cacheItem[T]{
		val:       val,
		expiredAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}
