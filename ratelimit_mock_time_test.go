package ratelimit

/**
This fake time implementation is a modification of time mocking
the mechanism used by Ian Lance Taylor in https://github.com/golang/time project
https://github.com/golang/time/commit/579cf78fd858857c0d766e0d63eb2b0ccf29f436

Modified parts:
 - timers are sorted on every addition, and then we relly of that order,
   we could use heap data structure, but sorting is OK for now.
 - advance accepts backoffDuration to sleep without lock held after every timer triggering
 - advanceUnlocked method yields the processor, after every timer triggering,
   allowing other goroutines to run
*/

import (
	"runtime"
	"sort"
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
func (tt *testTime) newTimer(dur time.Duration) (<-chan time.Time, func() bool) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	ch := make(chan time.Time, 1)
	timer := testTimer{
		when: tt.cur.Add(dur),
		ch:   ch,
	}
	tt.timers = append(tt.timers, timer)
	sort.Slice(tt.timers, func(i, j int) bool {
		return tt.timers[i].when.Before(tt.timers[j].when)
	})
	return ch, func() bool { return true }
}

// advance advances the fake time.
func (tt *testTime) advance(dur time.Duration, backoffDuration time.Duration) {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	targetTime := tt.cur.Add(dur)
	for {
		if len(tt.timers) == 0 || tt.timers[0].when.After(targetTime) {
			tt.cur = targetTime
			return
		}
		if tt.advanceUnlocked(tt.timers[0].when.Sub(tt.cur)) && backoffDuration > 0 {
			// after every timer triggering, we release our mutex
			// and give time for other goroutines to run
			tt.mu.Unlock()
			time.Sleep(backoffDuration)
			tt.mu.Lock()
		}
	}
}

// advanceUnlock advances the fake time, assuming it is already locked.
func (tt *testTime) advanceUnlocked(dur time.Duration) bool {
	tt.cur = tt.cur.Add(dur)
	if len(tt.timers) == 0 || tt.timers[0].when.After(tt.cur) {
		return false
	}

	i := 0
	for i < len(tt.timers) {
		if tt.timers[i].when.After(tt.cur) {
			break
		}
		tt.timers[i].ch <- tt.cur
		i++
		// calculate how many goroutines we currently have in runtime
		// and yield the processor, after every timer triggering,
		// allowing all other goroutines to run
		numOfAllRunningGoroutines := runtime.NumGoroutine()
		for j := 0; j < numOfAllRunningGoroutines; j++ {
			runtime.Gosched()
		}
	}

	tt.timers = tt.timers[i:]
	return true
}
