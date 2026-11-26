package retry

import (
	"context"
	"math/rand/v2"
	"time"
)

// Do executes fn up to attempts times with exponential backoff, jitter, and context cancellation.
func Do[T any](ctx context.Context, attempts int, baseDelay time.Duration, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if attempts < 1 {
		attempts = 1
	}
	if baseDelay <= 0 {
		baseDelay = 100 * time.Millisecond
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		val, err := fn(ctx)
		if err == nil {
			return val, nil
		}
		lastErr = err
		if attempt == attempts-1 {
			break
		}

		// Exponential backoff with +/-25% jitter
		delay := baseDelay << attempt
		jitter := time.Duration(rand.Int64N(int64(delay/2)+1)) - delay/4
		wait := max(delay+jitter, time.Millisecond)

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, ctx.Err()
		case <-timer.C:
		}
	}
	return zero, lastErr
}

// DoVoid runs an error-only function with exponential backoff and jitter.
func DoVoid(ctx context.Context, attempts int, baseDelay time.Duration, fn func(context.Context) error) error {
	_, err := Do(ctx, attempts, baseDelay, func(c context.Context) (struct{}, error) {
		return struct{}{}, fn(c)
	})
	return err
}
