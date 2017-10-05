package ratelimit_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/ratelimit"
	"go.uber.org/ratelimit/internal/clock"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/atomic"
	"context"
)

func ExampleRatelimit() {
	rl := ratelimit.New(100) // per second

	prev := time.Now()
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		rl.Take(ctx)
		now := time.Now()

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
	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		rl.Take(ctx)
	}
	assert.Condition(t, func() bool { return time.Now().Sub(now) < 1*time.Millisecond }, "no artificial delay")
}

func TestRateLimiter(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()

	clock := clock.NewMock()
	rl := ratelimit.New(100, ratelimit.WithClock(clock), ratelimit.WithoutSlack)

	count := atomic.NewInt32(0)

	// Until we're done...
	ctx, done := context.WithCancel(context.Background())
	defer done()

	// Create copious counts concurrently.
	go job(rl, count, ctx)
	go job(rl, count, ctx)
	go job(rl, count, ctx)
	go job(rl, count, ctx)

	clock.AfterFunc(1*time.Second, func() {
		assert.InDelta(t, 100, count.Load(), 10, "count within rate limit")
	})

	clock.AfterFunc(2*time.Second, func() {
		assert.InDelta(t, 200, count.Load(), 10, "count within rate limit")
	})

	clock.AfterFunc(3*time.Second, func() {
		assert.InDelta(t, 300, count.Load(), 10, "count within rate limit")
		wg.Done()
	})

	clock.Add(4 * time.Second)

	clock.Add(5 * time.Second)
}

func TestDelayedRateLimiter(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()

	clock := clock.NewMock()
	slow := ratelimit.New(10, ratelimit.WithClock(clock))
	fast := ratelimit.New(100, ratelimit.WithClock(clock))

	count := atomic.NewInt32(0)

	// Until we're done...
	ctx, done := context.WithCancel(context.Background())
	defer done()

	// Run a slow job
	go func() {
		for {
			slow.Take(ctx)
			fast.Take(ctx)
			count.Inc()
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	// Accumulate slack for 10 seconds,
	clock.AfterFunc(20*time.Second, func() {
		// Then start working.
		go job(fast, count, ctx)
		go job(fast, count, ctx)
		go job(fast, count, ctx)
		go job(fast, count, ctx)
	})

	clock.AfterFunc(30*time.Second, func() {
		assert.InDelta(t, 1200, count.Load(), 10, "count within rate limit")
		wg.Done()
	})

	clock.Add(40 * time.Second)
}

func job(rl ratelimit.Limiter, count *atomic.Int32, ctx context.Context) {
	for {
		rl.Take(ctx)
		count.Inc()
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}
