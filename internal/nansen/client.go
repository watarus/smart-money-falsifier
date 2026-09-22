package nansen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// ErrBudgetFloor is returned when a call would drop the remaining credit
// balance below the configured floor.
var ErrBudgetFloor = errors.New("nansen: call refused, would breach credit floor")

// ErrExpensiveEndpoint is returned when a call to a >1-credit endpoint is
// attempted without --allow-expensive.
var ErrExpensiveEndpoint = errors.New("nansen: expensive endpoint refused without --allow-expensive")

// ErrOffline is returned in offline mode when a cache miss would otherwise
// require an HTTP call.
var ErrOffline = errors.New("nansen: cache miss in offline mode")

const DefaultBaseURL = "https://api.nansen.ai/api/v1/"

// Client is a budget-aware, cached, rate-limited Nansen API client.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Cache      *Cache
	Limiter    *RateLimiter
	Log        *CallLog

	CreditFloor    int
	AllowExpensive bool
	Offline        bool
	MaxRetries     int

	mu               sync.Mutex
	creditsRemaining int // -1 means unknown (no response observed yet)
}

// NewClient builds a Client with the given options. Cache, Log and Limiter
// must be supplied by the caller (they're shared across the pipeline).
func NewClient(apiKey string, cache *Cache, limiter *RateLimiter, log *CallLog) *Client {
	return &Client{
		BaseURL:          DefaultBaseURL,
		APIKey:           apiKey,
		HTTPClient:       &http.Client{Timeout: 30 * time.Second},
		Cache:            cache,
		Limiter:          limiter,
		Log:              log,
		CreditFloor:      100,
		MaxRetries:       3,
		creditsRemaining: -1,
	}
}

// CreditsRemaining returns the last observed remaining-credit count, or -1
// if no live call has been observed yet.
func (c *Client) CreditsRemaining() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.creditsRemaining
}

func (c *Client) setCreditsRemaining(v int) {
	c.mu.Lock()
	c.creditsRemaining = v
	c.mu.Unlock()
}

// SeedCreditsRemaining primes the known remaining balance (e.g. from the
// last line of an existing calls.jsonl) so the budget guard is fail-closed
// even before this process has made a live call itself.
func (c *Client) SeedCreditsRemaining(v int) {
	c.setCreditsRemaining(v)
}

// reserveCredits checks the floor and, if the call is allowed, atomically
// deducts cost from the known remaining balance so concurrent workers
// can't all pass the check against the same stale value. reserved reports
// whether a deduction was made (false when the balance is still unknown,
// in which case the call proceeds unguarded — the only fail-open case).
func (c *Client) reserveCredits(cost int) (reserved bool, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.creditsRemaining < 0 {
		return false, nil
	}
	if c.creditsRemaining-cost < c.CreditFloor {
		return false, fmt.Errorf("%w: would leave %d credits (floor %d)", ErrBudgetFloor, c.creditsRemaining-cost, c.CreditFloor)
	}
	c.creditsRemaining -= cost
	return true, nil
}

func (c *Client) releaseReservedCredits(cost int) {
	c.mu.Lock()
	c.creditsRemaining += cost
	c.mu.Unlock()
}

// Plan describes whether a call would hit cache or require HTTP, and its
// declared credit cost, without performing any I/O.
type Plan struct {
	Endpoint    string
	CacheHit    bool
	CreditsCost int
}

// PlanCall reports whether path+body is already cached and its declared
// cost, without making any network request.
func (c *Client) PlanCall(path string, body any) (Plan, error) {
	cost := CreditCost[path]
	key, err := c.Cache.Key(path, body)
	if err != nil {
		return Plan{}, err
	}
	return Plan{Endpoint: path, CacheHit: c.Cache.Has(key), CreditsCost: cost}, nil
}

// Do executes a POST to path with body, decoding the JSON response into
// out. Responses are served from and written to the disk cache; a cache
// hit costs nothing and makes no network request. Returns whether the
// result came from cache.
func (c *Client) Do(ctx context.Context, path string, body any, out any) (cacheHit bool, err error) {
	key, err := c.Cache.Key(path, body)
	if err != nil {
		return false, fmt.Errorf("nansen: cache key: %w", err)
	}

	if raw, ok := c.Cache.Get(key); ok {
		if err := json.Unmarshal(raw, out); err != nil {
			return false, fmt.Errorf("nansen: decode cached %s: %w", path, err)
		}
		c.logAttempt(CallLogEntry{
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
			Endpoint:    path,
			Status:      200,
			CreditsCost: CreditCost[path],
			CacheHit:    true,
		})
		return true, nil
	}

	if c.Offline {
		return false, fmt.Errorf("%w: %s", ErrOffline, path)
	}

	cost := CreditCost[path]
	if cost > 1 && !c.AllowExpensive {
		return false, fmt.Errorf("%w: %s costs %d credits", ErrExpensiveEndpoint, path, cost)
	}
	reserved, err := c.reserveCredits(cost)
	if err != nil {
		return false, err
	}

	raw, entry, err := c.doHTTP(ctx, path, body, cost)
	c.logAttempt(entry)
	if err != nil {
		if reserved {
			// The call never completed (or failed outright); the guess we
			// reserved didn't actually get spent, so give it back. A
			// successful response instead overwrites creditsRemaining from
			// the authoritative header below.
			c.releaseReservedCredits(cost)
		}
		return false, err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return false, fmt.Errorf("nansen: decode response %s: %w", path, err)
	}
	if err := c.Cache.Put(key, raw); err != nil {
		return false, fmt.Errorf("nansen: cache put %s: %w", path, err)
	}
	return false, nil
}

func (c *Client) logAttempt(e CallLogEntry) {
	if c.Log == nil {
		return
	}
	_ = c.Log.Append(e)
}

// doHTTP performs the actual network call with rate limiting and retry
// logic. It returns the raw successful response body, or an error.
func (c *Client) doHTTP(ctx context.Context, path string, body any, cost int) ([]byte, CallLogEntry, error) {
	reqBytes, err := json.Marshal(body)
	if err != nil {
		return nil, CallLogEntry{}, fmt.Errorf("nansen: encode request %s: %w", path, err)
	}

	var lastEntry CallLogEntry
	var lastErr error

	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			wait := backoffDelay(attempt, lastEntry.retryAfter)
			t := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				t.Stop()
				return nil, lastEntry, ctx.Err()
			case <-t.C:
			}
		}

		if err := c.Limiter.Wait(ctx); err != nil {
			return nil, lastEntry, err
		}

		start := time.Now()
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(reqBytes))
		if err != nil {
			return nil, lastEntry, fmt.Errorf("nansen: build request %s: %w", path, err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("apikey", c.APIKey)

		resp, err := c.HTTPClient.Do(httpReq)
		duration := time.Since(start)
		if err != nil {
			lastErr = fmt.Errorf("nansen: request %s: %w", path, err)
			lastEntry = CallLogEntry{
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
				Endpoint:    path,
				Status:      0,
				CreditsCost: cost,
				DurationMS:  duration.Milliseconds(),
			}
			continue // network errors: retry like a 5xx
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		entry := CallLogEntry{
			Timestamp:        time.Now().UTC().Format(time.RFC3339),
			Endpoint:         path,
			Status:           resp.StatusCode,
			CreditsCost:      parseIntHeader(resp.Header, "X-Nansen-Credits-Cost"),
			CreditsUsed:      parseIntHeader(resp.Header, "X-Nansen-Credits-Used"),
			CreditsRemaining: parseIntHeader(resp.Header, "X-Nansen-Credits-Remaining"),
			RequestID:        resp.Header.Get("X-Request-Id"),
			DurationMS:       duration.Milliseconds(),
			retryAfter:       parseRetryAfter(resp.Header),
		}
		if resp.StatusCode == http.StatusOK {
			// Only a successful call actually spent credits; a 429/5xx
			// declared cost of 0 means exactly that, not "unknown".
			if entry.CreditsCost == 0 {
				entry.CreditsCost = cost
			}
			if v := resp.Header.Get("X-Nansen-Credits-Remaining"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					c.setCreditsRemaining(n)
				}
			}
		}

		if readErr != nil {
			lastErr = fmt.Errorf("nansen: read response %s: %w", path, readErr)
			lastEntry = entry
			continue
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			return respBody, entry, nil
		case resp.StatusCode == http.StatusTooManyRequests:
			lastErr = fmt.Errorf("nansen: %s: 429 rate limited", path)
			lastEntry = entry
			// retry loop applies Retry-After via lastEntry.retryAfter
			continue
		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("nansen: %s: server error %d", path, resp.StatusCode)
			lastEntry = entry
			continue
		default:
			// other 4xx: not retried
			return nil, entry, fmt.Errorf("nansen: %s: unexpected status %d: %s", path, resp.StatusCode, truncate(respBody, 500))
		}
	}
	return nil, lastEntry, fmt.Errorf("nansen: %s: exhausted retries: %w", path, lastErr)
}

func backoffDelay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	base := time.Duration(math.Pow(2, float64(attempt-1))) * 200 * time.Millisecond
	jitter := time.Duration(rand.Int63n(int64(base) + 1))
	return base + jitter
}

// maxRetryAfter caps how long a single server-supplied Retry-After can
// stall a worker; a hostile or misconfigured header can't hang a run.
const maxRetryAfter = 60 * time.Second

func parseRetryAfter(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return 0
	}
	var d time.Duration
	if secs, err := strconv.Atoi(v); err == nil {
		d = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(v); err == nil {
		d = time.Until(t)
	} else {
		return 0
	}
	if d > maxRetryAfter {
		return maxRetryAfter
	}
	if d < 0 {
		return 0
	}
	return d
}

func parseIntHeader(h http.Header, key string) int {
	v := h.Get(key)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
