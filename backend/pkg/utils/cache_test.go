package utils

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCache_GetMissReturnsNil(t *testing.T) {
	c := NewCache()
	if v := c.Get("nope"); v != nil {
		t.Errorf("expected nil for missing key, got %v", v)
	}
}

func TestCache_SetThenGetReturnsValue(t *testing.T) {
	c := NewCache()
	c.Set("k", "v", time.Minute)
	got := c.Get("k")
	if got != "v" {
		t.Errorf("Get(k) = %v, want v", got)
	}
}

func TestCache_SetWithDifferentTypes(t *testing.T) {
	c := NewCache()
	c.Set("str", "hello", time.Minute)
	c.Set("int", 42, time.Minute)
	c.Set("slice", []int{1, 2, 3}, time.Minute)

	if got := c.Get("str"); got != "hello" {
		t.Errorf("str: got %v", got)
	}
	if got := c.Get("int"); got != 42 {
		t.Errorf("int: got %v", got)
	}
	if got, ok := c.Get("slice").([]int); !ok || len(got) != 3 {
		t.Errorf("slice: got %v", c.Get("slice"))
	}
}

func TestCache_GetExpiredReturnsNil(t *testing.T) {
	c := NewCache()
	c.Set("k", "v", 10*time.Millisecond)
	time.Sleep(25 * time.Millisecond)
	if got := c.Get("k"); got != nil {
		t.Errorf("expected nil after TTL, got %v", got)
	}
}

func TestCache_DeleteRemovesItem(t *testing.T) {
	c := NewCache()
	c.Set("k", "v", time.Minute)
	c.Delete("k")
	if got := c.Get("k"); got != nil {
		t.Errorf("expected nil after Delete, got %v", got)
	}
}

func TestCache_DeleteMissingKeyIsNoOp(t *testing.T) {
	c := NewCache()
	// Should not panic or error.
	c.Delete("never-set")
}

func TestCache_ClearRemovesAllItems(t *testing.T) {
	c := NewCache()
	c.Set("a", 1, time.Minute)
	c.Set("b", 2, time.Minute)
	c.Set("c", 3, time.Minute)

	c.Clear()

	if c.Get("a") != nil || c.Get("b") != nil || c.Get("c") != nil {
		t.Errorf("expected all items cleared")
	}
}

func TestCache_SetOverwrites(t *testing.T) {
	c := NewCache()
	c.Set("k", "v1", time.Minute)
	c.Set("k", "v2", time.Minute)
	if got := c.Get("k"); got != "v2" {
		t.Errorf("expected v2 after overwrite, got %v", got)
	}
}

// TestCache_ConcurrentAccess exercises the RWMutex under load. Run with
// `go test -race` to catch data races. Uses bounded concurrency so the test
// stays deterministic.
func TestCache_ConcurrentAccess(t *testing.T) {
	c := NewCache()
	const goroutines = 50
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := range goroutines {
		go func(id int) {
			defer wg.Done()
			for i := range opsPerGoroutine {
				key := fmt.Sprintf("k%d", (id+i)%10)
				c.Set(key, i, time.Minute)
				_ = c.Get(key)
				if i%10 == 0 {
					c.Delete(key)
				}
			}
		}(g)
	}

	wg.Wait()
	// If we got here without a panic and `-race` is clean, the RWMutex is
	// protecting the map correctly.
}

// TestGlobalCache_IsUsable is a smoke test for the package-level var.
// It doesn't Clear() afterwards because the global is shared state that
// other packages may depend on at test time.
func TestGlobalCache_IsUsable(t *testing.T) {
	key := "stage2-smoke-key"
	GlobalCache.Set(key, "x", time.Minute)
	t.Cleanup(func() { GlobalCache.Delete(key) })
	if got := GlobalCache.Get(key); got != "x" {
		t.Errorf("GlobalCache.Get = %v, want x", got)
	}
}
