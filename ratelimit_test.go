package ratelimit

import (
	"sync"
	"testing"
	"time"

	"go.uber.org/atomic"

	"github.com/andres-erbsen/clock"
	"github.com/stretchr/testify/assert"
)

type testRunner interface {
	// createLimiter builds a limiter with given options.
	createLimiter(int, ...Option) Limiter
	// startTaking tries to Take() on passed in limiters in a loop/goroutine.
	startTaking(rls ...Limiter)
	// assertCountAt asserts the limiters have Taken() a number of times at the given time.
	// It's a thin wrapper around afterFunc to reduce boilerplate code.
	assertCountAt(d time.Duration, count int)
	// afterFunc executes a func at a given time.
	// not using clock.AfterFunc because andres-erbsen/clock misses a nap there.
	afterFunc(d time.Duration, fn func())
}

type runnerImpl struct {
	t *testing.T

	clock       *clock.Mock
	constructor func(int, ...Option) Limiter
	count       atomic.Int32
	// maxDuration is the time we need to move into the future for a test.
	// It's populated automatically based on assertCountAt/afterFunc.
	maxDuration time.Duration
	doneCh      chan struct{}
	wg          sync.WaitGroup
}

func runTest(t *testing.T, fn func(testRunner)) {
	impls := []struct {
		name        string
		constructor func(int, ...Option) Limiter
	}{
		{
			name: "mutex",
			constructor: func(rate int, opts ...Option) Limiter {
				return newMutexBased(rate, opts...)
			},
		},
		{
			name: "atomic",
			constructor: func(rate int, opts ...Option) Limiter {
				return newAtomicBased(rate, opts...)
			},
		},
	}

	for _, tt := range impls {
		t.Run(tt.name, func(t *testing.T) {
			r := runnerImpl{
				t:           t,
				clock:       clock.NewMock(),
				constructor: tt.constructor,
				doneCh:      make(chan struct{}),
			}
			defer close(r.doneCh)
			defer r.wg.Wait()

			fn(&r)
			r.clock.Add(r.maxDuration)
		})
	}
}

// createLimiter builds a limiter with given options.
func (r *runnerImpl) createLimiter(rate int, opts ...Option) Limiter {
	opts = append(opts, WithClock(r.clock))
	return r.constructor(rate, opts...)
}

// startTaking tries to Take() on passed in limiters in a loop/goroutine.
func (r *runnerImpl) startTaking(rls ...Limiter) {
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
		assert.Equal(r.t, int32(count), r.count.Load(), "count not as expected")
		r.wg.Done()
	})
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

func TestUnlimited(t *testing.T) {
	now := time.Now()
	rl := NewUnlimited()
	for i := 0; i < 1000; i++ {
		rl.Take()
	}
	assert.Condition(t, func() bool { return time.Since(now) < 1*time.Millisecond }, "no artificial delay")
}

func TestRateLimiter(t *testing.T) {
	runTest(t, func(r testRunner) {
		rl := r.createLimiter(100, WithoutSlack)

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
	runTest(t, func(r testRunner) {
		slow := r.createLimiter(10, WithoutSlack)
		fast := r.createLimiter(100, WithoutSlack)

		r.startTaking(slow, fast)

		r.afterFunc(20*time.Second, func() {
			r.startTaking(fast)
			r.startTaking(fast)
			r.startTaking(fast)
			r.startTaking(fast)
		})

		r.assertCountAt(30*time.Second, 1200)
	})
}

func TestPer(t *testing.T) {
	runTest(t, func(r testRunner) {
		rl := r.createLimiter(7, WithoutSlack, Per(time.Minute))

		r.startTaking(rl)
		r.startTaking(rl)

		r.assertCountAt(1*time.Second, 1)
		r.assertCountAt(1*time.Minute, 8)
		r.assertCountAt(2*time.Minute, 15)
	})
}

func TestSlack(t *testing.T) {
	// To simulate slack, we combine two limiters.
	// - First, we start a single goroutine with both of them,
	//   during this time the slow limiter will dominate,
	//   and allow the fast limiter to acumulate slack.
	// - After 2 seconds, we start another goroutine with
	//   only the faster limiter. This will allow it to max out,
	//   and consume all the slack.
	// - After 3 seconds, we look at the final result, and we expect,
	//   a sum of:
	//   - slower limiter running for 3 seconds
	//   - faster limiter running for 1 second
	//   - slack accumulated by the faster limiter during the two seconds.
	//     it was blocked by slower limiter.
	tests := []struct {
		msg  string
		opt  []Option
		want int
	}{
		{
			msg: "no option, defaults to 10",
			// 2*10 + 1*100 + 1*10 (slack)
			want: 130,
		},
		{
			msg: "slack of 10, like default",
			opt: []Option{WithSlack(10)},
			// 2*10 + 1*100 + 1*10 (slack)
			want: 130,
		},
		{
			msg: "slack of 20",
			opt: []Option{WithSlack(20)},
			// 2*10 + 1*100 + 1*20 (slack)
			want: 140,
		},
		{
			// Note this is bigger then the rate of the limiter.
			msg: "slack of 150",
			opt: []Option{WithSlack(150)},
			// 2*10 + 1*100 + 1*150 (slack)
			want: 270,
		},
		{
			msg: "no option, defaults to 10, with per",
			// 2*(10*2) + 1*(100*2) + 1*10 (slack)
			opt:  []Option{Per(500 * time.Millisecond)},
			want: 230,
		},
		{
			msg: "slack of 10, like default, with per",
			opt: []Option{WithSlack(10), Per(500 * time.Millisecond)},
			// 2*(10*2) + 1*(100*2) + 1*10 (slack)
			want: 230,
		},
		{
			msg: "slack of 20, with per",
			opt: []Option{WithSlack(20), Per(500 * time.Millisecond)},
			// 2*(10*2) + 1*(100*2) + 1*20 (slack)
			want: 240,
		},
		{
			// Note this is bigger then the rate of the limiter.
			msg: "slack of 150, with per",
			opt: []Option{WithSlack(150), Per(500 * time.Millisecond)},
			// 2*(10*2) + 1*(100*2) + 1*150 (slack)
			want: 370,
		},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			runTest(t, func(r testRunner) {
				slow := r.createLimiter(10, WithoutSlack)
				fast := r.createLimiter(100, tt.opt...)

				r.startTaking(slow, fast)

				r.afterFunc(2*time.Second, func() {
					r.startTaking(fast)
					r.startTaking(fast)
				})

				// limiter with 10hz dominates here - we're always at 10.
				r.assertCountAt(1*time.Second, 10)
				r.assertCountAt(3*time.Second, tt.want)
			})
		})
	}
}
