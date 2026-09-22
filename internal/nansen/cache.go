package nansen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// Cache is a disk-backed response cache keyed by sha256(path + canonical
// JSON body). It exists to guarantee that re-runs, tests, and demo
// recordings spend zero credits for work already done.
type Cache struct {
	Dir string
}

func NewCache(dir string) *Cache {
	return &Cache{Dir: dir}
}

// Key computes the cache key for a given endpoint path and request body.
// The body is canonicalised (keys sorted) before hashing so that
// semantically identical requests always hash the same regardless of
// struct field order.
func (c *Cache) Key(path string, body any) (string, error) {
	canon, err := canonicalJSON(body)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(append([]byte(path), canon...))
	return hex.EncodeToString(h[:]), nil
}

func (c *Cache) pathFor(key string) string {
	return filepath.Join(c.Dir, key+".json")
}

// Has reports whether a cached response exists for key.
func (c *Cache) Has(key string) bool {
	_, err := os.Stat(c.pathFor(key))
	return err == nil
}

// Get reads the cached raw response body for key, if present.
func (c *Cache) Get(key string) ([]byte, bool) {
	b, err := os.ReadFile(c.pathFor(key))
	if err != nil {
		return nil, false
	}
	return b, true
}

// Put stores the raw response body for key.
func (c *Cache) Put(key string, body []byte) error {
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return err
	}
	tmp := c.pathFor(key) + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.pathFor(key))
}

// canonicalJSON marshals v to JSON with map keys sorted recursively, giving
// a stable byte representation for hashing regardless of struct field
// order or map iteration order.
func canonicalJSON(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(b, &generic); err != nil {
		return nil, err
	}
	return marshalCanonical(generic)
}

func marshalCanonical(v any) ([]byte, error) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := []byte{'{'}
		for i, k := range keys {
			if i > 0 {
				out = append(out, ',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			out = append(out, kb...)
			out = append(out, ':')
			vb, err := marshalCanonical(t[k])
			if err != nil {
				return nil, err
			}
			out = append(out, vb...)
		}
		out = append(out, '}')
		return out, nil
	case []any:
		out := []byte{'['}
		for i, e := range t {
			if i > 0 {
				out = append(out, ',')
			}
			eb, err := marshalCanonical(e)
			if err != nil {
				return nil, err
			}
			out = append(out, eb...)
		}
		out = append(out, ']')
		return out, nil
	default:
		return json.Marshal(v)
	}
}
