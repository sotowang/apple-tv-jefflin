package cache

import (
	"testing"
	"time"
)

func TestExpiration(t *testing.T) {
	c := New[string, string](5 * time.Millisecond)
	c.Set("a", "value", 10*time.Millisecond)
	if v, ok := c.Get("a"); !ok || v != "value" {
		t.Fatal("cache miss")
	}
	time.Sleep(20 * time.Millisecond)
	if _, ok := c.Get("a"); ok {
		t.Fatal("expired entry returned")
	}
	c.Set("b", "value", time.Minute)
	c.Delete("b")
	if _, ok := c.Get("b"); ok {
		t.Fatal("delete failed")
	}
}
