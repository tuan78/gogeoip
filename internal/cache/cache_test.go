package cache

import (
	"context"
	"testing"
	"time"
)

// ── NoopCache ────────────────────────────────────────────────────────────────

func TestNoopCache_Get_AlwaysMisses(t *testing.T) {
	c := NoopCache{}
	val, ok := c.Get(context.Background(), "any-key")
	if ok {
		t.Error("Get: got ok=true, want false")
	}
	if val != "" {
		t.Errorf("Get: got val=%q, want empty string", val)
	}
}

func TestNoopCache_Get_MultipleCalls_AllMiss(t *testing.T) {
	c := NoopCache{}
	for i := 0; i < 3; i++ {
		val, ok := c.Get(context.Background(), "key")
		if ok || val != "" {
			t.Error("Get: expected miss on every call")
		}
	}
}

func TestNoopCache_Set_DoesNotPanic(_ *testing.T) {
	c := NoopCache{}
	c.Set(context.Background(), "key", "value", time.Hour)
}

func TestNoopCache_Set_MultipleCalls_NoEffect(t *testing.T) {
	c := NoopCache{}
	// Set multiple values
	c.Set(context.Background(), "key1", "value1", time.Hour)
	c.Set(context.Background(), "key2", "value2", 2*time.Hour)
	// Get should still miss (noops don't store)
	val, ok := c.Get(context.Background(), "key1")
	if ok || val != "" {
		t.Error("Set: expected noop to not store values")
	}
}

// ── RedisCache ───────────────────────────────────────────────────────────────

func TestNewRedisCache_FailsWhenServerUnreachable(t *testing.T) {
	_, err := NewRedisCache("localhost:1", "")
	if err == nil {
		t.Error("NewRedisCache: expected error for unreachable server, got nil")
	}
}
