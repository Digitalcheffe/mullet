package oauth

import (
	"testing"
	"time"
)

func TestPendingStoreBeginConsume(t *testing.T) {
	s := NewPendingStore()

	state, verifier, err := s.Begin(42)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if verifier == "" {
		t.Fatal("Begin returned an empty code verifier")
	}

	id, gotVerifier, ok := s.Consume(state)
	if !ok || id != 42 {
		t.Errorf("Consume(state) = (%d, _, %v), want (42, _, true)", id, ok)
	}
	if gotVerifier != verifier {
		t.Errorf("Consume returned verifier %q, want the one Begin issued (%q)", gotVerifier, verifier)
	}
}

func TestPendingStoreConsumeIsSingleUse(t *testing.T) {
	s := NewPendingStore()
	state, _, _ := s.Begin(1)

	if _, _, ok := s.Consume(state); !ok {
		t.Fatal("first Consume failed")
	}
	if _, _, ok := s.Consume(state); ok {
		t.Error("second Consume of the same state succeeded, want it rejected (replay)")
	}
}

func TestPendingStoreConsumeUnknownStateFails(t *testing.T) {
	s := NewPendingStore()
	if _, _, ok := s.Consume("never-issued"); ok {
		t.Error("Consume(unknown state) = true, want false")
	}
}

func TestPendingStoreConsumeExpiredStateFails(t *testing.T) {
	s := NewPendingStore()
	state, _, _ := s.Begin(1)
	// Reach into the store to simulate the TTL having already passed,
	// rather than sleeping the real 10 minutes in a test.
	s.mu.Lock()
	entry := s.entries[state]
	entry.expiresAt = time.Now().Add(-time.Second)
	s.entries[state] = entry
	s.mu.Unlock()

	if _, _, ok := s.Consume(state); ok {
		t.Error("Consume(expired state) = true, want false")
	}
}

func TestPendingStoreDistinctStatesForDifferentInstances(t *testing.T) {
	s := NewPendingStore()
	stateA, verifierA, _ := s.Begin(1)
	stateB, verifierB, _ := s.Begin(2)
	if stateA == stateB {
		t.Fatal("Begin returned the same state for two different instances")
	}
	if verifierA == verifierB {
		t.Fatal("Begin returned the same code verifier for two different instances")
	}

	idA, _, _ := s.Consume(stateA)
	idB, _, _ := s.Consume(stateB)
	if idA != 1 || idB != 2 {
		t.Errorf("consumed (idA, idB) = (%d, %d), want (1, 2)", idA, idB)
	}
}
