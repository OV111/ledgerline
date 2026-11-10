// Package ratelimit counts attempts per key (for example per IP address)
// and refuses them once a limit is reached within a time window.
//
// It is a "fixed window" counter: each key gets a window that starts at its
// first attempt. Inside the window at most `limit` attempts are allowed;
// when the window ends, the count starts again from zero.
//
// The data lives in memory, so it resets when the API restarts and is not
// shared between several API servers. That is fine for one demo server.
package ratelimit

import (
	"sync"
	"time"
)

type entry struct {
	start time.Time // when this key's current window began
	count int       // attempts inside the current window
}

type Limiter struct {
	limit  int
	window time.Duration

	// HTTP requests run at the same time in different goroutines, so every
	// access to the map must hold this lock. Without it the map can break.
	mu        sync.Mutex
	entries   map[string]*entry
	lastSweep time.Time

	// now returns the current time. Tests replace it with a fake clock so
	// they don't have to really wait a minute.
	now func() time.Time
}

// New makes a limiter that allows `limit` attempts per key per `window`.
func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]*entry),
		now:     time.Now,
	}
}

// Allow records one attempt for key.
// If the key is over the limit it returns false and how long to wait.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	e, found := l.entries[key]
	if !found || now.Sub(e.start) >= l.window {
		// First attempt, or the old window is over: start a new window.
		l.entries[key] = &entry{start: now, count: 1}
		return true, 0
	}
	if e.count >= l.limit {
		return false, e.start.Add(l.window).Sub(now)
	}
	e.count++
	return true, 0
}

// sweep deletes finished windows, at most once per window. Without it the
// map would keep one entry for every IP that ever visited and keep growing.
// The caller must hold l.mu.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	for key, e := range l.entries {
		if now.Sub(e.start) >= l.window {
			delete(l.entries, key)
		}
	}
	l.lastSweep = now
}
