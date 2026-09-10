package oauth

import (
	"sync"
	"time"
)

// pendingTTL bounds how long an admin has to complete the provider's
// consent screen after clicking Authorize before the state token
// expires and the callback is rejected -- generous, since a real
// enterprise tenant's admin-consent screen can involve its own delay.
const pendingTTL = 10 * time.Minute

// PendingStore tracks in-flight authorization attempts by their CSRF
// state token, so the callback can recover which plugin instance
// initiated it and reject a request whose state it doesn't recognize
// (forged, expired, or already used). It's deliberately in-memory, not
// persisted -- an attempt abandoned by a server restart just has to be
// retried from the Authorize button, which is harmless. Also carries each
// attempt's PKCE code verifier alongside its state token, since both are
// generated at the same Begin call and need to travel together to the one
// Consume call the callback makes.
type PendingStore struct {
	mu      sync.Mutex
	entries map[string]pendingEntry
}

type pendingEntry struct {
	instanceID   int
	codeVerifier string
	expiresAt    time.Time
}

func NewPendingStore() *PendingStore {
	return &PendingStore{entries: make(map[string]pendingEntry)}
}

// Begin registers a new in-flight attempt for instanceID and returns its
// state token and PKCE code verifier.
func (s *PendingStore) Begin(instanceID int) (state, codeVerifier string, err error) {
	state, err = NewState()
	if err != nil {
		return "", "", err
	}
	codeVerifier = NewCodeVerifier()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpiredLocked()
	s.entries[state] = pendingEntry{instanceID: instanceID, codeVerifier: codeVerifier, expiresAt: time.Now().Add(pendingTTL)}
	return state, codeVerifier, nil
}

// Consume validates and removes a state token, returning the plugin
// instance ID and PKCE code verifier it was issued for. Single-use: a
// state can't be replayed once consumed, whether the first consumption
// succeeded or not.
func (s *PendingStore) Consume(state string) (instanceID int, codeVerifier string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, found := s.entries[state]
	delete(s.entries, state)
	if !found || time.Now().After(entry.expiresAt) {
		return 0, "", false
	}
	return entry.instanceID, entry.codeVerifier, true
}

func (s *PendingStore) evictExpiredLocked() {
	now := time.Now()
	for state, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, state)
		}
	}
}
