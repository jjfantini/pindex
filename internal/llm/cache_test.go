package llm

import (
	"context"
	"testing"
)

func TestMemoryCacheHitAvoidsInner(t *testing.T) {
	inner := NewMock("m", MockResponse{Content: "a"}, MockResponse{Content: "b"})
	cp := NewCaching(inner, NewMemoryCache())
	r1, err := cp.Complete(context.Background(), UserPrompt("m", "q"))
	if err != nil {
		t.Fatal(err)
	}
	r2, err := cp.Complete(context.Background(), UserPrompt("m", "q"))
	if err != nil {
		t.Fatal(err)
	}
	if r1.Content != "a" || r2.Content != "a" {
		t.Errorf("got %q, %q want a, a (second served from cache)", r1.Content, r2.Content)
	}
	if inner.CallCount() != 1 {
		t.Errorf("inner calls=%d want 1", inner.CallCount())
	}
}

func TestCacheKeyStableAndDistinct(t *testing.T) {
	a := CacheKey(UserPrompt("m", "x"))
	if a != CacheKey(UserPrompt("m", "x")) {
		t.Error("key should be stable for identical requests")
	}
	if a == CacheKey(UserPrompt("m", "y")) {
		t.Error("key should differ by prompt")
	}
	if a == CacheKey(UserPrompt("n", "x")) {
		t.Error("key should differ by model")
	}
}

func TestFileCachePersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	fc, err := NewFileCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := fc.Set("k1", Response{Content: "hi", FinishReason: "stop"}); err != nil {
		t.Fatal(err)
	}
	fc2, err := NewFileCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := fc2.Get("k1")
	if !ok || r.Content != "hi" || r.FinishReason != "stop" {
		t.Errorf("persist failed: %+v ok=%v", r, ok)
	}
	if _, ok := fc2.Get("missing"); ok {
		t.Error("missing key should report absent")
	}
}
