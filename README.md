# Go rate limiter [![GoDoc][doc-img]][doc] [![Coverage Status][cov-img]][cov] ![test][test-img]

This package provides a Golang implementation of the leaky-bucket rate limit algorithm.
This implementation refills the bucket based on the time elapsed between
requests instead of requiring an interval clock to fill the bucket discretely.

Create a rate limiter with a maximum number of operations to perform per second.
Call Take() before each operation. Take will sleep until you can continue.

```go
import (
	"fmt"
	"time"

	"go.uber.org/ratelimit"
)

func main() {
    rl := ratelimit.New(100) // per second

    prev := time.Now()
    for i := 0; i < 10; i++ {
        now := rl.Take()
        fmt.Println(i, now.Sub(prev))
        prev = now
    }

    // Output:
    // 0 0
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
```

## Slack and Bursts

In a leaky-bucket rate limiter, requests are spaced evenly at intervals of `window / rate` (for example, every 10 ms for 100 requests per second).

By default, `ratelimit` allows a configurable **slack** buffer, which defaults to **10 requests** (`slack = 10`):

```go
rl := ratelimit.New(100) // slack defaults to 10
```

### How Slack Works

When an application is idle or requests arrive at a rate lower than configured, unspent time intervals accumulate up to a maximum slack allowance (`maxSlack = slack * perRequest`).

When subsequent calls to `Take()` occur after an idle period:
- Up to `slack` consecutive requests can return **immediately without sleeping**, consuming the accumulated unspent intervals as a burst.
- Once the accumulated slack is exhausted, subsequent calls return to regular pacing (e.g., 10 ms apart).

### Slack Configuration Options

- **Default Slack (`slack = 10`)**:
  Allows up to 10 requests to burst immediately after an idle period before enforcing rate intervals.
  ```go
  rl := ratelimit.New(100)
  ```

- **Custom Slack (`ratelimit.WithSlack(n)`)**:
  Configures custom slack up to `n` requests.
  ```go
  rl := ratelimit.New(100, ratelimit.WithSlack(5)) // burst up to 5 requests
  ```

- **Strict Pacing (`ratelimit.WithoutSlack`)**:
  Disables slack accumulation entirely (`slack = 0`). Unspent time during idle periods is discarded. The first call after an idle period returns immediately, and subsequent calls are strictly paced at `window / rate` (no bursts).
  ```go
  rl := ratelimit.New(100, ratelimit.WithoutSlack) // strict pacing, no bursts
  ```

### Example: Default Slack vs. WithoutSlack

With the default slack, an idle period allows subsequent calls to burst:

```go
rl := ratelimit.New(100) // default slack = 10 (10ms per request)

rl.Take()                         // Initial request
time.Sleep(time.Millisecond * 45) // Application was idle for 45ms (~4 request intervals)

prev := time.Now()
for i := 0; i < 6; i++ {
    now := rl.Take()
    if i > 0 {
        fmt.Println(i, now.Sub(prev).Round(time.Millisecond*2))
    }
    prev = now
}

// Output:
// 1 0s
// 2 0s
// 3 0s
// 4 4ms
// 5 10ms
```

With `ratelimit.WithoutSlack`, calls remain strictly paced at 10 ms intervals regardless of prior idle periods:

```go
rl := ratelimit.New(100, ratelimit.WithoutSlack)

rl.Take()                         // Initial request
time.Sleep(time.Millisecond * 45) // Application was idle

prev := time.Now()
for i := 0; i < 4; i++ {
    now := rl.Take()
    if i > 0 {
        fmt.Println(i, now.Sub(prev).Round(time.Millisecond*2))
    }
    prev = now
}

// Output:
// 1 10ms
// 2 10ms
// 3 10ms
```

## FAQ:
- Why does the rate limiter allow bursts of requests after an idle period? (based on #121, #4)

  By default, `ratelimit.New` configures a slack of 10 requests. When the limiter is idle, unspent intervals accumulate up to `maxSlack`, allowing up to 10 subsequent requests to proceed immediately without throttling. To enforce strict pacing with no bursts after idle periods, instantiate the limiter using `ratelimit.WithoutSlack`.

- What's the major diff v.s. https://pkg.go.dev/golang.org/x/time/rate? (based on #77)

  This ratelimiter was meant to have a (1) simple API and (2) minimal overhead. For more complex use-cases [x/time/rate] is a great choice.  See [here][redit] for historical context, and [here][bench] for benchmarks (from 2016).

- Why does example_test.go fail when I run it locally on Windows? (based on #80)

  Windows has some known issues with timers precision. See golang/go#44343. We don't expect to work around it.

[cov-img]: https://codecov.io/gh/uber-go/ratelimit/branch/master/graph/badge.svg?token=zhLeUjjrm2
[cov]: https://codecov.io/gh/uber-go/ratelimit
[doc-img]: https://pkg.go.dev/badge/go.uber.org/ratelimit
[doc]: https://pkg.go.dev/go.uber.org/ratelimit
[test-img]: https://github.com/uber-go/ratelimit/workflows/test/badge.svg
[redit]: https://www.reddit.com/r/golang/comments/59k2bi/ubergoratelimit_a_golang_blocking_leakybucket/d99ob9q
[x/time/rate]: https://pkg.go.dev/golang.org/x/time/rate
[bench]: https://gist.github.com/prashantv/26016a7dbc6fc1ec52d8c2b6591f3582
