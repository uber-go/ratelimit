package ratelimit

/**
This fake time implementation is a modification of time mocking
the mechanism used by Ian Lance Taylor in https://github.com/golang/time project
https://github.com/golang/time/commit/579cf78fd858857c0d766e0d63eb2b0ccf29f436

Modified parts:
 - advanceUnlocked method yields the processor, after every timer triggering,
   allowing other goroutines to run
*/

import (
	"runtime"
	"sync"
	"time"
)

// testTime is a fake time used for testing.
type testTime struct {
	mu     sync.Mutex
	cur    time.Time   // current fake time
	timers []testTimer // fake timers
}

// makeTestTime hooks the testTimer into the package.
func makeTestTime() *testTime {
	return &testTime{
		cur: time.Now(),
	}
}

// testTimer is a fake timer.
type testTimer struct {
	when time.Time
	ch   chan<- time.Time
}

// now returns the current fake time.
func (tt *testTime) now() time.Time {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	return tt.cur
}

// newTimer creates a fake timer. It returns the channel,
// a function to stop the timer (which we don't care about),
// and a function to advance to the next timer.
func (tt *testTime) newTimer(dur time.Duration) (<-chan time.Time, func() bool, func()) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	ch := make(chan time.Time, 1)
	timer := testTimer{
		when: tt.cur.Add(dur),
		ch:   ch,
	}
	tt.timers = append(tt.timers, timer)
	return ch, func() bool { return true }, tt.advanceToTimer
}

// advance advances the fake time.
func (tt *testTime) advance(dur time.Duration) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	tt.advanceUnlocked(dur)
}

// advanceUnlock advances the fake time, assuming it is already locked.
func (tt *testTime) advanceUnlocked(dur time.Duration) {
	tt.cur = tt.cur.Add(dur)
	i := 0
	for i < len(tt.timers) {
		if tt.timers[i].when.After(tt.cur) {
			i++
		} else {
			tt.timers[i].ch <- tt.cur
			// calculate how many goroutines we currently have in runtime
			// and yield the processor, after every timer triggering,
			// allowing all other goroutines to run
			numOfAllRunningGoroutines := runtime.NumGoroutine()
			for i := 0; i < numOfAllRunningGoroutines; i++ {
				runtime.Gosched()
			}
			copy(tt.timers[i:], tt.timers[i+1:])
			tt.timers = tt.timers[:len(tt.timers)-1]
		}
	}
}

// advanceToTimer advances the time to the next timer.
func (tt *testTime) advanceToTimer() {
	defer time.Sleep(2 * time.Millisecond)
	tt.mu.Lock()
	defer tt.mu.Unlock()
	if len(tt.timers) == 0 {
		return
	}
	when := tt.timers[0].when
	for _, timer := range tt.timers[1:] {
		if timer.when.Before(when) {
			when = timer.when
		}
	}
	tt.advanceUnlocked(when.Sub(tt.cur))
}
