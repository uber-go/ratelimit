// Copyright (c) 2016,2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package ratelimit // import "go.uber.org/ratelimit"

import (
	"time"

	"sync/atomic"
)

type atomicLimiter struct {
	state atomic.Pointer[state]

	//lint:ignore U1000 Padding is unused but it is crucial to maintain performance
	// of this rate limiter in case of collocation with other frequently accessed memory.
	padding [56]byte // cache line size - state pointer size = 64 - 8; created to avoid false sharing.

	perRequest time.Duration
	maxSlack   time.Duration
	clock      Clock
}

type state struct {
	last     time.Time
	sleepFor time.Duration
}

// newAtomicBased returns a new atomic based limiter.
func newAtomicBased(rate int, opts ...Option) *atomicLimiter {
	var al atomicLimiter
	al.init(rate, opts...)
	return &al
}

// init initialize a new atomic based limiter.
func (t *atomicLimiter) init(rate int, opts ...Option) {
	// TODO consider moving config building to the implementation
	// independent code.
	var config = buildConfig(opts)
	var perRequest = config.per / time.Duration(rate)

	t.perRequest = perRequest
	t.maxSlack = -1 * time.Duration(config.slack) * perRequest
	t.clock = config.clock
}

// Take blocks to ensure that the time spent between multiple
// Take calls is on average per/rate.
func (t *atomicLimiter) Take() time.Time {
	var (
		newState        state
		oldStatePointer *state
		taken           bool
		interval        time.Duration
	)
	for !taken {
		var now = t.clock.Now()

		oldStatePointer = t.state.Load()
		var oldState state
		if oldStatePointer != nil {
			oldState = *oldStatePointer
		}

		newState = state{
			last:     now,
			sleepFor: oldState.sleepFor,
		}

		// If this is our first request, then we allow it.
		if oldState.last.IsZero() {
			taken = t.state.CompareAndSwap(oldStatePointer, &newState)
			continue
		}

		// sleepFor calculates how much time we should sleep based on
		// the perRequest budget and how long the last request took.
		// Since the request may take longer than the budget, this number
		// can get negative, and is summed across requests.
		newState.sleepFor += t.perRequest - now.Sub(oldState.last)
		// We shouldn't allow sleepFor to get too negative, since it would mean that
		// a service that slowed down a lot for a short period of time would get
		// a much higher RPS following that.
		if newState.sleepFor < t.maxSlack {
			newState.sleepFor = t.maxSlack
		}
		if newState.sleepFor > 0 {
			newState.last = newState.last.Add(newState.sleepFor)
			interval, newState.sleepFor = newState.sleepFor, 0
		}
		taken = t.state.CompareAndSwap(oldStatePointer, &newState)
	}

	t.clock.Sleep(interval)
	return newState.last
}
