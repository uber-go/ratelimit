package ratelimit_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/uber-go/ratelimit/internal/clock"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/atomic"
	"github.com/uber-go/ratelimit"
)

func ExampleRatelimit() {
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

func TestRateLimiter(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	// clock := clock.New()
	// rl := ratelimit.New(100)

	clock := clock.NewMock()
	rl := ratelimit.NewWithClockWithoutSlack(100, clock)

	count := atomic.NewInt32(0)

	// Until we're done...
	done := make(chan struct{})
	defer close(done)

	// Create copious counts concurrently.
	go job(rl, count, done)
	go job(rl, count, done)
	go job(rl, count, done)
	go job(rl, count, done)

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

	wg.Wait()
}

func TestDelayedRateLimiter(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	// clock := clock.New()
	// rl := ratelimit.New(100)

	clock := clock.NewMock()
	rl := ratelimit.NewWithClockWithoutSlack(100, clock)

	count := atomic.NewInt32(0)

	// Until we're done...
	done := make(chan struct{})
	defer close(done)

	// Accumulate slack for 10 seconds,
	clock.AfterFunc(10*time.Second, func() {
		// Then start working.
		go job(rl, count, done)
		go job(rl, count, done)
		go job(rl, count, done)
		go job(rl, count, done)
	})

	clock.AfterFunc(20*time.Second, func() {
		assert.InDelta(t, 1000, count.Load(), 10, "count within rate limit")
		wg.Done()
	})

	clock.Add(30 * time.Second)

	wg.Wait()
}

func job(rl ratelimit.Limiter, count *atomic.Int32, done <-chan struct{}) {
	for {
		rl.Take()
		count.Inc()
		select {
		case <-done:
			return
		default:
		}
	}
}
