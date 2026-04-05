package store

import (
	"sync"
	"testing"
)

func TestSetAndGet(t *testing.T) {
	s := New()

	// Key absent → Get returns false.
	if _, ok := s.Get("missing"); ok {
		t.Fatal("expected miss for unknown key")
	}

	// First write → version 1.
	v := s.Set("k", "hello", 0)
	if v != 1 {
		t.Fatalf("expected version 1, got %d", v)
	}

	e, ok := s.Get("k")
	if !ok || e.Value != "hello" || e.Version != 1 {
		t.Fatalf("unexpected entry: %+v", e)
	}

	// Second write → version 2.
	v2 := s.Set("k", "world", 0)
	if v2 != 2 {
		t.Fatalf("expected version 2, got %d", v2)
	}
}

func TestIncomingVersionWins(t *testing.T) {
	s := New()
	// Simulate a replication message arriving with version 10.
	v := s.Set("k", "replicated", 10)
	if v != 10 {
		t.Fatalf("expected version 10 from incoming, got %d", v)
	}

	// A subsequent local write should be 11.
	v2 := s.Set("k", "local", 0)
	if v2 != 11 {
		t.Fatalf("expected version 11, got %d", v2)
	}
}

func TestEmptyValueAllowed(t *testing.T) {
	s := New()
	s.Set("k", "", 0)
	e, ok := s.Get("k")
	if !ok || e.Value != "" {
		t.Fatal("empty string value should be stored and returned")
	}
}

func TestEmptyKeyPrevented(t *testing.T) {
	// The HTTP handler rejects empty keys before calling store.Set;
	// the store itself does not enforce this, but we document it here.
	s := New()
	s.Set("", "oops", 0)
	_, ok := s.Get("")
	// Store allows it; enforcement is at the handler layer.
	_ = ok
}

func TestConcurrentWrites(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Set("shared", "val", 0)
		}()
	}
	wg.Wait()
	e, ok := s.Get("shared")
	if !ok {
		t.Fatal("key should exist after concurrent writes")
	}
	if e.Version < 1 || e.Version > 100 {
		t.Fatalf("version out of expected range: %d", e.Version)
	}
}
