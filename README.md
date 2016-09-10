# Go rate limiter

This package provides a Golang implementation of the leaky-bucket rate limit algorithm.
This implementation refills the bucket based on the time elapsed between
requests instead of requiring an interval clock to fill the bucket discretely.
