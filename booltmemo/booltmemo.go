// Package booltmemo provides functionality to memoize boolean functions
// with different Time-To-Live (TTL) values for true and false results.
package booltmemo

import (
	"sync"
	"time"
)

// CacheEntry represents a cached boolean result with its expiration time.
type CacheEntry struct {
	Value     bool
	ExpiresAt time.Time
}

// Memoizer stores the memoized function and its cache.
type Memoizer[T any] struct {
	fn           func(T) bool
	cache        map[any]CacheEntry
	mutex        sync.RWMutex
	trueTTL      time.Duration
	falseTTL     time.Duration
	cleanupTimer *time.Timer
}

// New creates a new Memoizer for the given boolean function with specified TTLs.
// - fn: The function to memoize that takes any value and returns a boolean
// - trueTTL: How long to cache 'true' results
// - falseTTL: How long to cache 'false' results
func New[T any](fn func(T) bool, trueTTL, falseTTL time.Duration) *Memoizer[T] {
	m := &Memoizer[T]{
		fn:       fn,
		cache:    make(map[any]CacheEntry),
		trueTTL:  trueTTL,
		falseTTL: falseTTL,
	}

	// Set up periodic cleanup of expired entries
	m.startCleanupTimer()

	return m
}

// startCleanupTimer starts a timer to periodically clean up expired cache entries.
func (m *Memoizer[T]) startCleanupTimer() {
	// Find the minimum TTL to determine cleanup frequency
	minTTL := m.trueTTL
	if m.falseTTL < minTTL {
		minTTL = m.falseTTL
	}

	// Use a reasonable cleanup interval based on the shortest TTL
	cleanupInterval := minTTL / 2
	if cleanupInterval < time.Second {
		cleanupInterval = time.Second
	}

	m.cleanupTimer = time.AfterFunc(cleanupInterval, func() {
		m.cleanup()
		// Safely restart the timer
		m.mutex.Lock()
		if m.cleanupTimer != nil { // Check if stopped before restarting
			m.startCleanupTimer() // Restart the timer
		}
		m.mutex.Unlock()
	})
}

// cleanup removes expired entries from the cache.
func (m *Memoizer[T]) cleanup() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	for key, entry := range m.cache {
		if now.After(entry.ExpiresAt) {
			delete(m.cache, key)
		}
	}
}

// Get retrieves the cached result for the given key, or computes and caches it.
func (m *Memoizer[T]) Get(key T) bool {
	// Try to get from cache first
	m.mutex.RLock()
	entry, found := m.cache[key]
	m.mutex.RUnlock()

	// If found and not expired, return the cached value
	if found && time.Now().Before(entry.ExpiresAt) {
		return entry.Value
	}

	// Otherwise, compute the result
	return m.compute(key)
}

// compute calls the underlying function and caches the result with appropriate TTL.
// It handles concurrent calls safely.
func (m *Memoizer[T]) compute(key T) bool {
	// Acquire full lock for computation and cache update
	m.mutex.Lock()

	// Double-check: Another goroutine might have computed this while we waited for the lock
	entry, found := m.cache[key]
	if found && time.Now().Before(entry.ExpiresAt) {
		m.mutex.Unlock()
		return entry.Value // Return the value computed by the other goroutine
	}

	// If still not found or expired, proceed with computation
	result := m.fn(key)

	// Determine TTL based on result
	ttl := m.falseTTL
	if result {
		ttl = m.trueTTL
	}

	// Cache the result
	expiresAt := time.Now().Add(ttl)
	m.cache[key] = CacheEntry{
		Value:     result,
		ExpiresAt: expiresAt,
	}

	m.mutex.Unlock()

	return result
}

// Invalidate removes a specific key from the cache.
func (m *Memoizer[T]) Invalidate(key T) {
	m.mutex.Lock()
	delete(m.cache, key)
	m.mutex.Unlock()
}

// Clear removes all entries from the cache.
func (m *Memoizer[T]) Clear() {
	m.mutex.Lock()
	m.cache = make(map[any]CacheEntry)
	m.mutex.Unlock()
}

// Stop halts the cleanup timer.
func (m *Memoizer[T]) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.cleanupTimer != nil {
		m.cleanupTimer.Stop()
		m.cleanupTimer = nil // Prevent further access after stop
	}
}
