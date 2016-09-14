package clock

// Forked from github.com/andres-erbsen/clock to isolate a missing nap.

import (
	"sort"
	"sync"
	"time"
)

// Mock represents a mock clock that only moves forward programmically.
// It can be preferable to a real-time clock when testing time-based functionality.
type Mock struct {
	sync.Mutex
	now    time.Time // current time
	timers Timers    // timers
}

// NewMock returns an instance of a mock clock.
// The current time of the mock clock on initialization is the Unix epoch.
func NewMock() *Mock {
	return &Mock{now: time.Unix(0, 0)}
}

// Add moves the current time of the mock clock forward by the duration.
// This should only be called from a single goroutine at a time.
func (m *Mock) Add(d time.Duration) {
	m.Lock()
	// Calculate the final time.
	end := m.now.Add(d)

	for len(m.timers) > 0 && m.now.Before(end) {
		// TODO use a heap instead of resorting an array every iteration.
		// (Advantage of this approach is that the timers are in stable order.)
		sort.Stable(m.timers)
		t := m.timers[0]
		m.now = t.next
		m.Unlock()
		t.Tick()
		m.Lock()
		m.timers = m.timers[1:]
	}

	m.Unlock()
	// Give a small buffer to make sure the other goroutines get handled.
	nap()
}

// Produces a timer that will emit a time some duration after now.
func (m *Mock) Timer(d time.Duration) *Timer {
	ch := make(chan time.Time, 0)
	t := &Timer{
		C:    ch,
		c:    ch,
		mock: m,
		next: m.now.Add(d),
	}
	m.addTimer(t)
	return t
}

func (m *Mock) addTimer(t *Timer) {
	m.Lock()
	defer m.Unlock()
	// Lazy about sorting.
	m.timers = append(m.timers, t)
}

func (m *Mock) After(d time.Duration) <-chan time.Time {
	return m.Timer(d).C
}

// AfterFunc waits for the duration to elapse and then executes a function.
// A Timer is returned that can be stopped.
func (m *Mock) AfterFunc(d time.Duration, f func()) *Timer {
	t := m.Timer(d)
	go func() {
		<-t.c
		f()
	}()
	nap()
	return t
}

// Now returns the current wall time on the mock clock.
func (m *Mock) Now() time.Time {
	return m.now
}

// Sleep pauses the goroutine for the given duration on the mock clock.
// The clock must be moved forward in a separate goroutine.
func (m *Mock) Sleep(d time.Duration) {
	<-m.After(d)
}

// timers represents a list of sortable timers.
type Timers []*Timer

func (a Timers) Len() int { return len(a) }
func (a Timers) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a Timers) Less(i, j int) bool { return a[i].Next().Before(a[j].Next()) }

// Timer represents a single event.
// The current time will be sent on C, unless the timer was created by AfterFunc.
type Timer struct {
	C    <-chan time.Time
	c    chan time.Time
	next time.Time // next tick time
	mock *Mock     // mock clock
}

func (t *Timer) Next() time.Time { return t.next }

func (t *Timer) Tick() {
	select {
	case t.c <- t.next:
	default:
	}
	nap()
}

// Sleep momentarily so that other goroutines can process.
func nap() { time.Sleep(1 * time.Millisecond) }
