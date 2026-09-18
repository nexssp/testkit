package testkit

import (
	"sync"
	"time"
)

// FakeClock is a deterministic replacement for time.Now / time.After /
// time.NewTicker. Install it into a context with WithClock, and code
// that reads from ctx can be driven forward in tests with Advance.
//
// This is what lets a test verify "the retry waited 200ms" without a
// real 200ms sleep.
type FakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []*clockWaiter
}

type clockWaiter struct {
	until   time.Time
	ready   chan struct{}
	stopped bool
}

// NewFakeClock returns a clock frozen at the given instant.
func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{now: start}
}

// Now returns the current fake time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward, waking every waiter whose deadline
// has passed. Waiters are woken in deadline order.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	now := c.now
	var ready []*clockWaiter
	remaining := c.waiters[:0]
	for _, w := range c.waiters {
		if w.stopped {
			continue
		}
		if !w.until.After(now) {
			ready = append(ready, w)
			continue
		}
		remaining = append(remaining, w)
	}
	c.waiters = remaining
	c.mu.Unlock()

	for _, w := range ready {
		close(w.ready)
	}
}

// After returns a channel that fires when the clock has advanced past d.
func (c *FakeClock) After(d time.Duration) <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	w := &clockWaiter{
		until: c.now.Add(d),
		ready: make(chan struct{}),
	}
	c.waiters = append(c.waiters, w)
	return w.ready
}
