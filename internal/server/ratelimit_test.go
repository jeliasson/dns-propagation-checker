package server

import (
	"testing"
	"time"
)

// clock is a manually advanced time source.
type clock struct{ t time.Time }

func (c *clock) now() time.Time      { return c.t }
func (c *clock) add(d time.Duration) { c.t = c.t.Add(d) }

func newTestLimiter(burst int, window time.Duration) (*limiter, *clock) {
	c := &clock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := newLimiter(burst, window)
	l.now = c.now
	return l, c
}

func TestLimiterAllowsBurstThenBlocks(t *testing.T) {
	l, _ := newTestLimiter(3, time.Minute)

	for i := range 3 {
		if !l.allow("a") {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}
	if l.allow("a") {
		t.Error("request past the burst should have been blocked")
	}
}

func TestLimiterRefillsOverTime(t *testing.T) {
	l, c := newTestLimiter(6, time.Minute) // one token every 10s

	for range 6 {
		l.allow("a")
	}
	if l.allow("a") {
		t.Fatal("bucket should be empty")
	}

	c.add(9 * time.Second)
	if l.allow("a") {
		t.Error("not enough time has passed for a token")
	}

	c.add(2 * time.Second)
	if !l.allow("a") {
		t.Error("a token should have refilled after 11s")
	}
}

func TestLimiterDoesNotExceedBurstAfterLongIdle(t *testing.T) {
	l, c := newTestLimiter(3, time.Minute)

	l.allow("a")
	c.add(time.Hour)

	for i := range 3 {
		if !l.allow("a") {
			t.Fatalf("request %d should have been allowed after idling", i+1)
		}
	}
	if l.allow("a") {
		t.Error("idling must not grant more than the burst")
	}
}

func TestLimiterKeysAreIndependent(t *testing.T) {
	l, _ := newTestLimiter(1, time.Minute)

	if !l.allow("a") || !l.allow("b") {
		t.Fatal("different keys should have separate budgets")
	}
	if l.allow("a") {
		t.Error("key a should be exhausted")
	}
}

func TestLimiterSweepDropsIdleBuckets(t *testing.T) {
	l, c := newTestLimiter(5, time.Minute)

	l.allow("a")
	c.add(30 * time.Second)
	l.allow("b")

	// Only "a" has been idle long enough to have fully refilled.
	c.add(31 * time.Second)
	l.sweep()

	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.buckets["a"]; ok {
		t.Error("idle bucket a should have been swept")
	}
	if _, ok := l.buckets["b"]; !ok {
		t.Error("recently used bucket b should have been kept")
	}
}

func TestLimiterSweepIsSafeWhenEmpty(t *testing.T) {
	l, _ := newTestLimiter(5, time.Minute)
	l.sweep() // must not panic or divide by zero
}
