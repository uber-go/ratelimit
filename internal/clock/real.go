package clock

import "time"

// clock implements a real-time clock by simply wrapping the time package functions.
type clock struct{}

// New returns an instance of a real-time clock.
func New() Clock {
	return &clock{}
}

func (c *clock) After(d time.Duration) <-chan time.Time { return time.After(d) }

func (c *clock) AfterFunc(d time.Duration, f func()) {
	// TODO maybe return timer interface
	time.AfterFunc(d, f)
}

func (c *clock) Now() time.Time { return time.Now() }

func (c *clock) Sleep(d time.Duration) { time.Sleep(d) }
