package oauth

import (
	"testing"
	"time"
)

func TestPendingStoreBeginConsume(t *testing.T) {
	s := NewPendingStore()

	state, err := s.Begin(42)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	id, ok := s.Consume(state)
	if !ok || id != 42 {
		t.Errorf("Consume(state) = (%d, %v), want (42, true)", id, ok)
	}
}

func TestPendingStoreConsumeIsSingleUse(t *testing.T) {
	s := NewPendingStore()
	state, _ := s.Begin(1)

	if _, ok := s.Consume(state); !ok {
		t.Fatal("first Consume failed")
	}
	if _, ok := s.Consume(state); ok {
		t.Error("second Consume of the same state succeeded, want it rejected (replay)")
	}
}

func TestPendingStoreConsumeUnknownStateFails(t *testing.T) {
	s := NewPendingStore()
	if _, ok := s.Consume("never-issued"); ok {
		t.Error("Consume(unknown state) = true, want false")
	}
}

func TestPendingStoreConsumeExpiredStateFails(t *testing.T) {
	s := NewPendingStore()
	state, _ := s.Begin(1)
	// Reach into the store to simulate the TTL having already passed,
	// rather than sleeping the real 10 minutes in a test.
	s.mu.Lock()
	entry := s.entries[state]
	entry.expiresAt = time.Now().Add(-time.Second)
	s.entries[state] = entry
	s.mu.Unlock()

	if _, ok := s.Consume(state); ok {
		t.Error("Consume(expired state) = true, want false")
	}
}

func TestPendingStoreDistinctStatesForDifferentInstances(t *testing.T) {
	s := NewPendingStore()
	stateA, _ := s.Begin(1)
	stateB, _ := s.Begin(2)
	if stateA == stateB {
		t.Fatal("Begin returned the same state for two different instances")
	}

	idA, _ := s.Consume(stateA)
	idB, _ := s.Consume(stateB)
	if idA != 1 || idB != 2 {
		t.Errorf("consumed (idA, idB) = (%d, %d), want (1, 2)", idA, idB)
	}
}
