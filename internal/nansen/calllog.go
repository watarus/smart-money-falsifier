package nansen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CallLogEntry is one line of out/calls.jsonl: evidence of every attempted
// API call, used to satisfy and reconcile the >=1000-call requirement.
type CallLogEntry struct {
	Timestamp        string `json:"ts"`
	Endpoint         string `json:"endpoint"`
	Status           int    `json:"status"`
	CreditsCost      int    `json:"credits_cost"`
	CreditsUsed      int    `json:"credits_used"`
	CreditsRemaining int    `json:"credits_remaining"`
	RequestID        string `json:"request_id"`
	CacheHit         bool   `json:"cache_hit"`
	DurationMS       int64  `json:"duration_ms"`

	// retryAfter carries the server's requested backoff between retry
	// attempts within doHTTP; it is never logged.
	retryAfter time.Duration
}

// CallLog appends JSON-lines call records to a file, safe for concurrent
// use by the pipeline's worker pool.
type CallLog struct {
	mu   sync.Mutex
	f    *os.File
}

func NewCallLog(path string) (*CallLog, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &CallLog{f: f}, nil
}

func (c *CallLog) Append(e CallLogEntry) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err = c.f.Write(b)
	return err
}

func (c *CallLog) Close() error {
	if c.f == nil {
		return nil
	}
	return c.f.Close()
}
