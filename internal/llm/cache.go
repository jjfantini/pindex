package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// CacheKey is a stable hash of the request's model, messages, and temperature —
// the basis for the prompt-hash response cache that makes re-runs and crash
// recovery nearly free and doubles as the deterministic test cassette key.
func CacheKey(req Request) string {
	h := sha256.New()
	_ = json.NewEncoder(h).Encode(struct {
		Model    string
		Messages []Message
		Temp     float64
	}{req.Model, req.Messages, req.Temperature})
	return hex.EncodeToString(h.Sum(nil))
}

// Cache stores completions keyed by CacheKey.
type Cache interface {
	Get(key string) (Response, bool)
	Set(key string, resp Response) error
}

// MemoryCache is an in-process Cache.
type MemoryCache struct {
	mu sync.Mutex
	m  map[string]Response
}

// NewMemoryCache returns an empty MemoryCache.
func NewMemoryCache() *MemoryCache { return &MemoryCache{m: make(map[string]Response)} }

// Get implements Cache.
func (c *MemoryCache) Get(key string) (Response, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.m[key]
	return r, ok
}

// Set implements Cache.
func (c *MemoryCache) Set(key string, r Response) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = r
	return nil
}

// FileCache persists completions as one JSON file per key under a directory.
type FileCache struct{ dir string }

// NewFileCache creates (if needed) and returns a directory-backed cache.
func NewFileCache(dir string) (*FileCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &FileCache{dir: dir}, nil
}

func (c *FileCache) path(key string) string { return filepath.Join(c.dir, key+".json") }

// Get implements Cache.
func (c *FileCache) Get(key string) (Response, bool) {
	b, err := os.ReadFile(c.path(key))
	if err != nil {
		return Response{}, false
	}
	var r Response
	if json.Unmarshal(b, &r) != nil {
		return Response{}, false
	}
	return r, true
}

// Set implements Cache.
func (c *FileCache) Set(key string, r Response) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(c.path(key), b, 0o644)
}

// CachingProvider is a read-through cache in front of a Provider. A cache write
// failure does not fail the request (the live result is still returned).
type CachingProvider struct {
	inner Provider
	cache Cache
	// Observer, when set, receives EventCacheHit/EventCacheMiss per request.
	Observer Observer
}

// NewCaching wraps inner with cache.
func NewCaching(inner Provider, cache Cache) *CachingProvider {
	return &CachingProvider{inner: inner, cache: cache}
}

// Name implements Provider.
func (p *CachingProvider) Name() string { return p.inner.Name() }

// Complete implements Provider, returning a cached response on hit.
func (p *CachingProvider) Complete(ctx context.Context, req Request) (Response, error) {
	key := CacheKey(req)
	if r, ok := p.cache.Get(key); ok {
		p.Observer.emit(Event{Kind: EventCacheHit, Provider: p.inner.Name(), Model: req.Model})
		return r, nil
	}
	p.Observer.emit(Event{Kind: EventCacheMiss, Provider: p.inner.Name(), Model: req.Model})
	r, err := p.inner.Complete(ctx, req)
	if err != nil {
		return Response{}, err
	}
	_ = p.cache.Set(key, r)
	return r, nil
}
