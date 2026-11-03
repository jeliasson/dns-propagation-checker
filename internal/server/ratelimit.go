package server

import (
	"sync"
	"time"
)

// limiter is a per-key token bucket: each key gets burst tokens that refill
// at rate tokens per second. It keeps no state beyond what is actively in use.
type limiter struct {
	rate  float64 // tokens per second
	burst float64

	mu      sync.Mutex
	buckets map[string]*bucket
	now     func() time.Time // injectable for tests
}

type bucket struct {
	tokens float64
	seen   time.Time
}

// newLimiter allows burst requests immediately, then one more every
// window/burst until the bucket refills.
func newLimiter(burst int, window time.Duration) *limiter {
	if burst < 1 {
		burst = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &limiter{
		rate:    float64(burst) / window.Seconds(),
		burst:   float64(burst),
		buckets: make(map[string]*bucket),
		now:     time.Now,
	}
}

func (l *limiter) allow(key string) bool {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, seen: now}
		l.buckets[key] = b
	}

	b.tokens = min(l.burst, b.tokens+now.Sub(b.seen).Seconds()*l.rate)
	b.seen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// sweep drops buckets that have been idle long enough to have fully refilled,
// since those are indistinguishable from a fresh one.
func (l *limiter) sweep() {
	idle := time.Duration(l.burst/l.rate) * time.Second
	cutoff := l.now().Add(-idle)

	l.mu.Lock()
	defer l.mu.Unlock()

	for key, b := range l.buckets {
		if b.seen.Before(cutoff) {
			delete(l.buckets, key)
		}
	}
}

// sweepEvery runs sweep on a ticker until done is closed.
func (l *limiter) sweepEvery(interval time.Duration, done <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.sweep()
		case <-done:
			return
		}
	}
}
