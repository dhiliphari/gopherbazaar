package retry

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"
)

// Policy describes how to retry.
type Policy struct {
	// MaxAttempts is the total number of calls, including the first.
	// Values below 1 are treated as 1 (no retries).
	MaxAttempts int

	// BaseDelay is the upper bound of the wait before the first retry.
	// Each later retry doubles the bound, up to MaxDelay.
	BaseDelay time.Duration

	// MaxDelay caps the upper bound of any single wait.
	MaxDelay time.Duration

	// Retryable reports whether an error is worth retrying. If nil, every
	// error is retried. Return false for errors where trying again can't
	// help ("insufficient funds") or might hurt.
	Retryable func(error) bool
}

// Backoff returns how long to wait before retry number n (0 for the first
// retry). It uses "full jitter": a uniformly random duration between zero
// and min(MaxDelay, BaseDelay × 2ⁿ).
func (p Policy) Backoff(n int) time.Duration {
	ceiling := p.BaseDelay
	for i := 0; i < n && ceiling < p.MaxDelay; i++ {
		ceiling *= 2 // doubling in a loop can't overflow: we stop at MaxDelay
	}
	if p.MaxDelay > 0 && ceiling > p.MaxDelay {
		ceiling = p.MaxDelay
	}
	if ceiling <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(ceiling)))
}

// Do calls fn until it succeeds, returns an error that isn't retryable, the
// attempts run out, or ctx is done. It never waits past ctx's deadline: if
// the next wait wouldn't fit, it stops and returns the last error.
func Do(ctx context.Context, p Policy, fn func(context.Context) error) error {
	attempts := max(p.MaxAttempts, 1)

	var err error
	for attempt := range attempts {
		if attempt > 0 {
			wait := p.Backoff(attempt - 1)
			if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < wait {
				return fmt.Errorf("retry: not enough time left for attempt %d: %w", attempt+1, err)
			}
			if werr := sleep(ctx, wait); werr != nil {
				return fmt.Errorf("retry: %w (last error: %w)", werr, err)
			}
		}

		err = fn(ctx)
		if err == nil {
			return nil
		}
		if p.Retryable != nil && !p.Retryable(err) {
			return err
		}
	}
	return fmt.Errorf("retry: gave up after %d attempts: %w", attempts, err)
}

// sleep waits for d, or until ctx is done, whichever comes first.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
