package clock

import "time"

// Clock represents an interface to the functions in the standard library time
// package. Two implementations are available in the clock package. The first
// is a real-time clock which simply wraps the time package's functions. The
// second is a mock clock which will only make forward progress when
// programmatically adjusted.
type Clock interface {
	AfterFunc(d time.Duration, f func())
	Now() time.Time
	Sleep(d time.Duration)
}
