// Package cache is a small in-process TTL cache for read-only tool results.
// Concurrent misses for one key share a single upstream load (singleflight),
// and entries older than half their TTL are served stale while one
// background refresh runs.
package cache

import (
	"context"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const refreshTimeout = 30 * time.Second

type entry struct {
	val        any
	at         time.Time
	refreshing bool
}

// Cache keys are "<profile>/<kind>/<id>".
type Cache struct {
	now func() time.Time
	sf  singleflight.Group
	mu  sync.RWMutex
	m   map[string]*entry
}

func New(now func() time.Time) *Cache {
	if now == nil {
		now = time.Now
	}
	return &Cache{now: now, m: map[string]*entry{}}
}

// Get returns the cached value for key or loads it.
func (c *Cache) Get(ctx context.Context, key string, ttl time.Duration, load func(context.Context) (any, error)) (any, error) {
	c.mu.RLock()
	e, ok := c.m[key]
	var val any
	var age time.Duration
	if ok {
		val, age = e.val, c.now().Sub(e.at)
	}
	c.mu.RUnlock()

	switch {
	case ok && age < ttl/2:
		return val, nil
	case ok && age < ttl:
		c.refreshInBackground(ctx, key, load)
		return val, nil
	}
	v, err, _ := c.sf.Do(key, func() (any, error) {
		v, err := load(ctx)
		if err == nil {
			c.store(key, v)
		}
		return v, err
	})
	return v, err
}

func (c *Cache) refreshInBackground(ctx context.Context, key string, load func(context.Context) (any, error)) {
	c.mu.Lock()
	e, ok := c.m[key]
	if !ok || e.refreshing {
		c.mu.Unlock()
		return
	}
	e.refreshing = true
	c.mu.Unlock()

	bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), refreshTimeout)
	go func() {
		defer cancel()
		_, _, _ = c.sf.Do(key, func() (any, error) {
			v, err := load(bg)
			if err == nil {
				c.store(key, v)
			} else {
				c.mu.Lock()
				if e, ok := c.m[key]; ok {
					e.refreshing = false
				}
				c.mu.Unlock()
			}
			return v, err
		})
	}()
}

func (c *Cache) store(key string, v any) {
	c.mu.Lock()
	c.m[key] = &entry{val: v, at: c.now()}
	c.mu.Unlock()
}

// Invalidate drops every key with the given prefix.
func (c *Cache) Invalidate(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.m {
		if strings.HasPrefix(k, prefix) {
			delete(c.m, k)
		}
	}
}
