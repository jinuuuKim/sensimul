package clock

import (
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
	Since(time.Time) time.Duration
	TickDuration() time.Duration
	Accelerated() bool
	Advance(time.Duration)
}

type RealClock struct {
	tickInterval time.Duration
}

func NewReal(tickInterval time.Duration) *RealClock {
	return &RealClock{tickInterval: tickInterval}
}

func (c *RealClock) Now() time.Time {
	return time.Now()
}

func (c *RealClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

func (c *RealClock) TickDuration() time.Duration {
	return c.tickInterval
}

func (c *RealClock) Accelerated() bool {
	return false
}

func (c *RealClock) Advance(d time.Duration) {
}

type SimClock struct {
	mu           sync.Mutex
	current      time.Time
	tickInterval time.Duration
}

func NewSim(start time.Time, tickInterval time.Duration) *SimClock {
	return &SimClock{
		current:      start,
		tickInterval: tickInterval,
	}
}

func (c *SimClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

func (c *SimClock) Since(t time.Time) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current.Sub(t)
}

func (c *SimClock) TickDuration() time.Duration {
	return c.tickInterval
}

func (c *SimClock) Accelerated() bool {
	return true
}

func (c *SimClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = c.current.Add(d)
}
