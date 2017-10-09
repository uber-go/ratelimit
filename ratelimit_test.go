package ratelimit_test

import (
	"fmt"
	"sync"
	"testing"
	"time"
	"context"

	"go.uber.org/ratelimit"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/atomic"
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

	rl := ratelimit.New(100, ratelimit.WithoutSlack)

	count := atomic.NewInt32(0)

	// Until we're done...
	ctx := context.Background()

	// Create copious counts concurrently.
	go job(rl, count, ctx)
	go job(rl, count, ctx)
	go job(rl, count, ctx)
	go job(rl, count, ctx)

	time.AfterFunc(1*time.Second, func() {
		assert.InDelta(t, 100, count.Load(), 10, "count within rate limit")
	})

	time.AfterFunc(2*time.Second, func() {
		assert.InDelta(t, 200, count.Load(), 10, "count within rate limit")
	})

	time.AfterFunc(3*time.Second, func() {
		assert.InDelta(t, 300, count.Load(), 10, "count within rate limit")
		wg.Done()
	})
}

func TestDelayedRateLimiter(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()

	slow := ratelimit.New(10)
	fast := ratelimit.New(100)

	count := atomic.NewInt32(0)

	// Until we're done...
	ctx := context.Background()

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
	time.AfterFunc(20*time.Second, func() {
		// Then start working.
		go job(fast, count, ctx)
		go job(fast, count, ctx)
		go job(fast, count, ctx)
		go job(fast, count, ctx)
	})

	time.AfterFunc(30*time.Second, func() {
		assert.InDelta(t, 1200, count.Load(), 10, "count within rate limit")
		wg.Done()
	})
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
