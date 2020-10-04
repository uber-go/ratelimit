package alternatives

import (
	"fmt"
	"runtime"
	"sync"
	"testing"

	"go.uber.org/atomic"

	"go.uber.org/ratelimit"
	atomicr "go.uber.org/ratelimit/internal/alternatives/atomic"
	"go.uber.org/ratelimit/internal/alternatives/mutex"
)

func BenchmarkRateLimiter(b *testing.B) {
	count := atomic.NewInt64(0)
	for _, procs := range []int{1, 4, 8, 16} {
		runtime.GOMAXPROCS(procs)
		for name, limiter := range map[string]ratelimit.Limiter{
			"atomic": atomicr.New(b.N * 10000000),
			"mutex":  mutex.New(b.N * 10000000),
		} {
			for ng := 1; ng < 16; ng++ {
				runner(b, name, procs, ng, limiter, count)
			}
			for ng := 16; ng < 128; ng += 8 {
				runner(b, name, procs, ng, limiter, count)
			}
			for ng := 128; ng < 512; ng += 16 {
				runner(b, name, procs, ng, limiter, count)
			}
			for ng := 512; ng < 1024; ng += 32 {
				runner(b, name, procs, ng, limiter, count)
			}
			for ng := 1024; ng < 2048; ng += 8 {
				runner(b, name, procs, ng, limiter, count)
			}
			for ng := 2048; ng < 4096; ng += 128 {
				runner(b, name, procs, ng, limiter, count)
			}
			for ng := 4096; ng < 16384; ng += 512 {
				runner(b, name, procs, ng, limiter, count)
			}
			for ng := 16384; ng < 65536; ng += 2048 {
				runner(b, name, procs, ng, limiter, count)
			}
		}
	}
	fmt.Printf("\nmark%d\n", count.Load())
}

func runner(b *testing.B, name string, procs int, ng int, limiter ratelimit.Limiter, count *atomic.Int64) bool {
	return b.Run(fmt.Sprintf("type:%s-procs:%d-goroutines:%d", name, procs, ng), func(b *testing.B) {
		var wg sync.WaitGroup
		trigger := atomic.NewBool(true)
		n := b.N
		batchSize := n / ng
		if batchSize == 0 {
			batchSize = n
		}
		for n > 0 {
			wg.Add(1)
			batch := min(n, batchSize)
			n -= batch
			go func(quota int) {
				for trigger.Load() {
					runtime.Gosched()
				}
				localCnt := 0
				for i := 0; i < quota; i++ {
					res := limiter.Take()
					localCnt += res.Nanosecond()
				}
				count.Add(int64(localCnt))
				wg.Done()
			}(batch)
		}

		b.StartTimer()
		trigger.Store(false)
		wg.Wait()
		b.StopTimer()
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
