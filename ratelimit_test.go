package ratelimit_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/atomic"
	"go.uber.org/ratelimit"

	"github.com/andres-erbsen/clock"
	"github.com/stretchr/testify/assert"
)

type runner interface {
	// startTaking tries to Take() on passed in limiters in a loop/goroutine.
	startTaking(rls ...ratelimit.Limiter)
	// assertCountAt asserts the limiters have Taken() a number of times at the given time.
	// It's a thin wrapper around afterFunc to reduce boilerplate code.
	assertCountAt(d time.Duration, count int)
	// getClock returns the test clock.
	getClock() ratelimit.Clock
	// afterFunc executes a func at a given time.
	// not using clock.AfterFunc because andres-erbsen/clock misses a nap there.
	afterFunc(d time.Duration, fn func())
}

type runnerImpl struct {
	t *testing.T

	clock *clock.Mock
	count atomic.Int32
	// maxDuration is the time we need to move into the future for a test.
	// It's populated automatically based on assertCountAt/afterFunc.
	maxDuration time.Duration
	doneCh      chan struct{}
	wg          sync.WaitGroup
}

func runTest(t *testing.T, fn func(runner)) {
	r := runnerImpl{
		t:      t,
		clock:  clock.NewMock(),
		doneCh: make(chan struct{}),
	}
	defer close(r.doneCh)
	defer r.wg.Wait()

	fn(&r)
	r.clock.Add(r.maxDuration)
}

// startTaking tries to Take() on passed in limiters in a loop/goroutine.
func (r *runnerImpl) startTaking(rls ...ratelimit.Limiter) {
	r.goWait(func() {
		for {
			for _, rl := range rls {
				rl.Take()
			}
			r.count.Inc()
			select {
			case <-r.doneCh:
				return
			default:
			}
		}
	})
}

// assertCountAt asserts the limiters have Taken() a number of times at a given time.
func (r *runnerImpl) assertCountAt(d time.Duration, count int) {
	r.wg.Add(1)
	r.afterFunc(d, func() {
		assert.InDelta(r.t, count, r.count.Load(), 10, "count within rate limit")
		r.wg.Done()
	})
}

// getClock return the test clock.
func (r *runnerImpl) getClock() ratelimit.Clock {
	return r.clock
}

// afterFunc executes a func at a given time.
func (r *runnerImpl) afterFunc(d time.Duration, fn func()) {
	if d > r.maxDuration {
		r.maxDuration = d
	}

	r.goWait(func() {
		select {
		case <-r.doneCh:
			return
		case <-r.clock.After(d):
		}
		fn()
	})
}

// goWait runs a function in a goroutine and makes sure the gouritine was scheduled.
func (r *runnerImpl) goWait(fn func()) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Done()
		fn()
	}()
	wg.Wait()
}

func Example() {
	rl := ratelimit.New(100) // per second

	prev := time.Now()
	for i := 0; i < 10; i++ {
		now := rl.Take()
		if i > 0 {
			fmt.Println(i, now.Sub(prev))
		}
		prev = now
	}

	// Output:
	// 1 10ms
	// 2 10ms
	// 3 10ms
	// 4 10ms
	// 5 10ms
	// 6 10ms
	// 7 10ms
	// 8 10ms
	// 9 10ms
}

func TestUnlimited(t *testing.T) {
	now := time.Now()
	rl := ratelimit.NewUnlimited()
	for i := 0; i < 1000; i++ {
		rl.Take()
	}
	assert.Condition(t, func() bool { return time.Since(now) < 1*time.Millisecond }, "no artificial delay")
}

func TestRateLimiter(t *testing.T) {
	runTest(t, func(r runner) {
		rl := ratelimit.New(100, ratelimit.WithClock(r.getClock()), ratelimit.WithoutSlack)

		// Create copious counts concurrently.
		r.startTaking(rl)
		r.startTaking(rl)
		r.startTaking(rl)
		r.startTaking(rl)

		r.assertCountAt(1*time.Second, 100)
		r.assertCountAt(2*time.Second, 200)
		r.assertCountAt(3*time.Second, 300)
	})
}

func TestDelayedRateLimiter(t *testing.T) {
	runTest(t, func(r runner) {
		slow := ratelimit.New(10, ratelimit.WithClock(r.getClock()))
		fast := ratelimit.New(100, ratelimit.WithClock(r.getClock()))

		// Run a slow startTaking
		r.startTaking(slow, fast)

		// Accumulate slack for 10 seconds,
		r.afterFunc(20*time.Second, func() {
			// Then start working.
			r.startTaking(fast)
			r.startTaking(fast)
			r.startTaking(fast)
			r.startTaking(fast)
		})

		r.assertCountAt(30*time.Second, 1200)
	})
}
