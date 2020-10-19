package ratelimit_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/atomic"
	"go.uber.org/ratelimit"
	"go.uber.org/ratelimit/internal/clock"

	"github.com/stretchr/testify/assert"
)

type runner struct {
	wg     sync.WaitGroup
	clock  *clock.Mock
	count  atomic.Int32
	doneCh chan struct{}
}

func runTest(t *testing.T, fn func(runner)) {
	r := runner{
		clock:  clock.NewMock(),
		doneCh: make(chan struct{}),
	}
	defer close(r.doneCh)
	defer r.wg.Wait()

	fn(r)
}

func (r *runner) job(rl ratelimit.Limiter) {
	go func() {
		for {
			rl.Take()
			r.count.Inc()
			select {
			case <-r.doneCh:
				return
			default:
			}
		}
	}()
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
		rl := ratelimit.New(100, ratelimit.WithClock(r.clock), ratelimit.WithoutSlack)

		// Create copious counts concurrently.
		r.job(rl)
		r.job(rl)
		r.job(rl)
		r.job(rl)

		r.clock.AfterFunc(1*time.Second, func() {
			assert.InDelta(t, 100, r.count.Load(), 10, "count within rate limit")
		})

		r.clock.AfterFunc(2*time.Second, func() {
			assert.InDelta(t, 200, r.count.Load(), 10, "count within rate limit")
		})

		r.wg.Add(1)
		r.clock.AfterFunc(3*time.Second, func() {
			assert.InDelta(t, 300, r.count.Load(), 10, "count within rate limit")
			r.wg.Done()
		})

		r.clock.Add(4 * time.Second)
	})

}

func TestDelayedRateLimiter(t *testing.T) {
	runTest(t, func(r runner) {
		slow := ratelimit.New(10, ratelimit.WithClock(r.clock))
		fast := ratelimit.New(100, ratelimit.WithClock(r.clock))

		// Run a slow job
		go func() {
			for {
				slow.Take()
				fast.Take()
				r.count.Inc()
				select {
				case <-r.doneCh:
					return
				default:
				}
			}
		}()

		// Accumulate slack for 10 seconds,
		r.clock.AfterFunc(20*time.Second, func() {
			// Then start working.
			r.job(fast)
			r.job(fast)
			r.job(fast)
			r.job(fast)
		})

		r.wg.Add(1)
		r.clock.AfterFunc(30*time.Second, func() {
			assert.InDelta(t, 1200, r.count.Load(), 10, "count within rate limit")
			r.wg.Done()
		})

		r.clock.Add(40 * time.Second)
	})

}
