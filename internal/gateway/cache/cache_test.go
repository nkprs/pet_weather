package cache

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	ttl := 150 * time.Millisecond

	c := New[int](ttl)

	if c == nil {
		t.Fatal("expected non-nil cache")
	}

	if c.ttl != ttl {
		t.Fatalf("expected ttl %v, got %v", ttl, c.ttl)
	}

	if c.items == nil {
		t.Fatal("expected items map to be initialized")
	}

	if len(c.items) != 0 {
		t.Fatalf("expected empty map, got len=%d", len(c.items))
	}
}

func TestGetSet(t *testing.T) {
	ttl := 150 * time.Millisecond
	c := New[int](ttl)

	v, ok := c.Get("foo")
	if ok {
		t.Fatalf("ok expected false, got %v", ok)
	}
	if v != 0 {
		t.Fatalf("value expected 0, got %v", v)
	}

	c.Set("foo", 10)
	v, ok = c.Get("foo")
	if !ok {
		t.Fatalf("ok expected true, got %v", ok)
	}
	if v != 10 {
		t.Fatalf("value expected 10, got %v", v)
	}

	time.Sleep(ttl * 2)
	v, ok = c.Get("foo")
	if ok {
		t.Fatalf("ok expected false, got %v", ok)
	}
	if v != 0 {
		t.Fatalf("value expected 0, got %v", v)
	}

}
