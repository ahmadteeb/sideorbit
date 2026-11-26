// Package ratelimit is a small keyed token bucket.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter allows n events per window for each key, with bursts up to n.
//
// shortcut: state is in memory, so limits apply per replica and reset on
// restart. Move to a shared store when running more than one server.
type Limiter struct {
	n      int
	window time.Duration
	now    func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
	swept   time.Time
}

type bucket struct {
	lim  *rate.Limiter
	seen time.Time
}

// New returns a limiter of n events per window. n <= 0 refuses everything.
func New(n int, window time.Duration) *Limiter {
	return &Limiter{n: max(n, 0), window: window, now: time.Now, buckets: map[string]*bucket{}}
}

// Allow records one event for key and reports whether it is within the limit.
func (l *Limiter) Allow(key string) bool {
	if l.n == 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	// A bucket idle for a whole window is full again, so it can be dropped.
	if now.Sub(l.swept) >= l.window {
		for k, b := range l.buckets {
			if now.Sub(b.seen) >= l.window {
				delete(l.buckets, k)
			}
		}
		l.swept = now
	}
	b := l.buckets[key]
	if b == nil {
		b = &bucket{lim: rate.NewLimiter(rate.Every(l.window/time.Duration(l.n)), l.n)}
		l.buckets[key] = b
	}
	b.seen = now
	return b.lim.AllowN(now, 1)
}
