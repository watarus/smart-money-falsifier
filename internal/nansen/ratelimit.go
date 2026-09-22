package nansen

import (
	"context"
	"sync"
	"time"
)

// tokenBucket is a simple thread-safe token bucket rate limiter.
type tokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	rate     float64 // tokens per second
	last     time.Time
	now      func() time.Time
}

func newTokenBucket(ratePerSec float64, capacity float64) *tokenBucket {
	return &tokenBucket{
		tokens:   capacity,
		capacity: capacity,
		rate:     ratePerSec,
		last:     time.Now(),
		now:      time.Now,
	}
}

// Wait blocks until a token is available or ctx is cancelled.
func (b *tokenBucket) Wait(ctx context.Context) error {
	for {
		b.mu.Lock()
		now := b.now()
		elapsed := now.Sub(b.last).Seconds()
		if elapsed > 0 {
			b.tokens += elapsed * b.rate
			if b.tokens > b.capacity {
				b.tokens = b.capacity
			}
			b.last = now
		}
		if b.tokens >= 1 {
			b.tokens--
			b.mu.Unlock()
			return nil
		}
		wait := time.Duration((1 - b.tokens) / b.rate * float64(time.Second))
		b.mu.Unlock()
		if wait <= 0 {
			wait = time.Millisecond
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

// RateLimiter combines a per-second and a per-minute token bucket; a
// request must acquire a token from both before proceeding, keeping
// throughput under both limits simultaneously.
type RateLimiter struct {
	perSecond *tokenBucket
	perMinute *tokenBucket
}

// NewRateLimiter builds a limiter under the given per-second and per-minute
// caps. Defaults (12 req/s, 250 req/min) are set by the caller to stay
// under the Free tier's 15/300.
func NewRateLimiter(perSec, perMin int) *RateLimiter {
	return &RateLimiter{
		perSecond: newTokenBucket(float64(perSec), float64(perSec)),
		perMinute: newTokenBucket(float64(perMin)/60, float64(perMin)),
	}
}

func (r *RateLimiter) Wait(ctx context.Context) error {
	if err := r.perSecond.Wait(ctx); err != nil {
		return err
	}
	return r.perMinute.Wait(ctx)
}
