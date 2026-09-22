package nansen

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	cache := NewCache(filepath.Join(dir, "cache"))
	limiter := NewRateLimiter(1000, 60000)
	logPath := filepath.Join(dir, "calls.jsonl")
	log, err := NewCallLog(logPath)
	if err != nil {
		t.Fatalf("NewCallLog: %v", err)
	}
	t.Cleanup(func() { log.Close() })

	c := NewClient("test-key", cache, limiter, log)
	c.BaseURL = srv.URL + "/"
	c.MaxRetries = 3
	return c, &hits
}

func TestCacheHitMakesNoHTTPCall(t *testing.T) {
	var calls int32
	c, hits := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("X-Nansen-Credits-Cost", "1")
		w.Header().Set("X-Nansen-Credits-Used", "1")
		w.Header().Set("X-Nansen-Credits-Remaining", "500")
		w.Header().Set("X-Request-Id", "req-1")
		json.NewEncoder(w).Encode(PnLSummaryResponse{WinRate: 0.5})
	})

	ctx := t.Context()
	req := PnLSummaryRequest{Address: "0xabc", Chain: "ethereum"}

	var out1 PnLSummaryResponse
	hit1, err := c.Do(ctx, PathProfilerPnLSummary, req, &out1)
	if err != nil {
		t.Fatalf("first Do: %v", err)
	}
	if hit1 {
		t.Fatalf("expected first call to be a live call, got cache hit")
	}
	if atomic.LoadInt32(hits) != 1 {
		t.Fatalf("expected 1 server hit, got %d", *hits)
	}

	var out2 PnLSummaryResponse
	hit2, err := c.Do(ctx, PathProfilerPnLSummary, req, &out2)
	if err != nil {
		t.Fatalf("second Do: %v", err)
	}
	if !hit2 {
		t.Fatalf("expected second call to be served from cache")
	}
	if atomic.LoadInt32(hits) != 1 {
		t.Fatalf("expected server hits to remain 1 after cache hit, got %d", *hits)
	}
	if out2.WinRate != 0.5 {
		t.Fatalf("cached response decoded wrong: %+v", out2)
	}
}

func TestBudgetFloorRefused(t *testing.T) {
	c, hits := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Nansen-Credits-Remaining", "50")
		json.NewEncoder(w).Encode(PnLSummaryResponse{})
	})
	c.CreditFloor = 100

	ctx := t.Context()
	// First call succeeds and reports remaining=50, which is already below
	// the floor + cost, but the client only knows this *after* the call.
	var out PnLSummaryResponse
	if _, err := c.Do(ctx, PathProfilerPnLSummary, PnLSummaryRequest{Address: "0x1"}, &out); err != nil {
		t.Fatalf("priming call: %v", err)
	}

	// Second call (different address => different cache key) must now be
	// refused before any HTTP request, since remaining(50) - cost(1) < floor(100).
	before := atomic.LoadInt32(hits)
	_, err := c.Do(ctx, PathProfilerPnLSummary, PnLSummaryRequest{Address: "0x2"}, &out)
	if err == nil {
		t.Fatalf("expected budget floor error, got nil")
	}
	if atomic.LoadInt32(hits) != before {
		t.Fatalf("expected no additional HTTP call, hits went from %d to %d", before, atomic.LoadInt32(hits))
	}
}

func TestExpensiveEndpointRefusedWithoutFlag(t *testing.T) {
	c, hits := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Nansen-Credits-Remaining", "1000")
		json.NewEncoder(w).Encode(DexTradesResponse{})
	})

	ctx := t.Context()
	var out DexTradesResponse
	_, err := c.Do(ctx, PathSmartMoneyDexTrades, DexTradesRequest{Chains: []string{"all"}}, &out)
	if err == nil {
		t.Fatalf("expected expensive-endpoint refusal, got nil")
	}
	if atomic.LoadInt32(hits) != 0 {
		t.Fatalf("expected no HTTP call, got %d", *hits)
	}

	c.AllowExpensive = true
	_, err = c.Do(ctx, PathSmartMoneyDexTrades, DexTradesRequest{Chains: []string{"all"}}, &out)
	if err != nil {
		t.Fatalf("expected success with --allow-expensive, got %v", err)
	}
	if atomic.LoadInt32(hits) != 1 {
		t.Fatalf("expected 1 HTTP call after allowing, got %d", *hits)
	}
}

func TestRetryAfter429ThenSucceeds(t *testing.T) {
	var attempt int32
	c, hits := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempt, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("X-Nansen-Credits-Remaining", "1000")
		json.NewEncoder(w).Encode(PnLSummaryResponse{WinRate: 0.9})
	})

	ctx := t.Context()
	var out PnLSummaryResponse
	start := time.Now()
	hit, err := c.Do(ctx, PathProfilerPnLSummary, PnLSummaryRequest{Address: "0xretry"}, &out)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if hit {
		t.Fatalf("did not expect cache hit")
	}
	if out.WinRate != 0.9 {
		t.Fatalf("unexpected result: %+v", out)
	}
	if atomic.LoadInt32(hits) != 2 {
		t.Fatalf("expected 2 attempts (429 then success), got %d", *hits)
	}
	if elapsed < 900*time.Millisecond {
		t.Fatalf("expected the client to honour Retry-After: 1s, only waited %v", elapsed)
	}
}

func TestCallLogLinesAreWellFormedJSON(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Nansen-Credits-Cost", "1")
		w.Header().Set("X-Nansen-Credits-Used", "1")
		w.Header().Set("X-Nansen-Credits-Remaining", "999")
		w.Header().Set("X-Request-Id", "req-xyz")
		json.NewEncoder(w).Encode(PnLSummaryResponse{})
	})

	ctx := t.Context()
	var out PnLSummaryResponse
	if _, err := c.Do(ctx, PathProfilerPnLSummary, PnLSummaryRequest{Address: "0xlog"}, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if _, err := c.Do(ctx, PathProfilerPnLSummary, PnLSummaryRequest{Address: "0xlog"}, &out); err != nil {
		t.Fatalf("Do (cached): %v", err)
	}
	c.Log.Close()

	data, err := os.ReadFile(c.Log.f.Name())
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := splitNonEmptyLines(data)
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(lines))
	}
	for i, line := range lines {
		var e CallLogEntry
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatalf("line %d not valid JSON: %v", i, err)
		}
		if e.Endpoint != PathProfilerPnLSummary {
			t.Fatalf("line %d wrong endpoint: %+v", i, e)
		}
		if e.Timestamp == "" {
			t.Fatalf("line %d missing timestamp: %+v", i, e)
		}
	}
	if !mustDecodeCacheHit(t, lines[0]) && !mustDecodeCacheHit(t, lines[1]) {
		t.Fatalf("expected exactly one cache_hit:true line")
	}
}

func mustDecodeCacheHit(t *testing.T, line []byte) bool {
	t.Helper()
	var e CallLogEntry
	if err := json.Unmarshal(line, &e); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return e.CacheHit
}

func splitNonEmptyLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	return lines
}
