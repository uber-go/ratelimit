package ratelimit

import (
	"sync"
	"testing"
	"time"

	"github.com/andres-erbsen/clock"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/atomic"
)

func TestRateLimiter(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	clock := clock.NewMock()
	count := atomic.NewInt32(0)
	rl := NewWithClock(100, clock)

	// Until we're done...
	done := make(chan struct{})
	defer close(done)

	// Generate copious counts.
	go func() {
		for {
			rl.Take()
			count.Inc()
			select {
			case <-done:
				return
			default:
			}
		}
	}()

	clock.AfterFunc(time.Second, func() {
		assert.InDelta(t, 100, count.Load(), 10, "count within rate limit")
	})

	clock.AfterFunc(time.Duration(2)*time.Second, func() {
		assert.InDelta(t, 200, count.Load(), 10, "count within rate limit")
		wg.Done()
	})

	for i := 0; i < 10; i++ {
		clock.Add(time.Duration(i*100) * time.Millisecond)
	}

	wg.Wait()
}
