package store

import (
	"sync"
	"time"
	"tracemind/internal/models"
	"tracemind/internal/util"
)

type testStore struct {
	mu      sync.RWMutex
	signals map[string]models.Signal
}

func NewStore() *testStore {
	return &testStore{
		signals: make(map[string]models.Signal),
	}
}

func (s *testStore) SaveSignal(sig models.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sig.ID == "" {
		sig.ID = util.GenID()
	}
	if sig.Timestamp.IsZero() {
		sig.Timestamp = time.Now().UTC()
	}
	sig.Message = SanitizeMessage(sig.Message)
	sig.Payload = RedactPayloadByAllowList(sig.Payload, payloadAllowListSnapshot())
	s.signals[sig.ID] = sig
}

func (s *testStore) GetSignal(id string) (models.Signal, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sig, ok := s.signals[id]
	return sig, ok
}

func (s *testStore) DeleteSignalsOlderThan(cutoff time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	deleted := 0
	for id, sig := range s.signals {
		if sig.Timestamp.Before(cutoff) {
			delete(s.signals, id)
			deleted++
		}
	}
	return deleted
}
