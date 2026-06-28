package store

import (
	"sync"
	"time"

	"tracemind/internal/models"
	"tracemind/internal/util"
)

// Storage defines the persistence contract used by workers and API
type Storage interface {
	SaveSignal(models.Signal)
	GetSignal(string) (models.Signal, bool)
	DeleteSignalsOlderThan(time.Time) int
	SaveIncident(models.Incident)
	ListIncidents() []models.Incident
	GetIncident(string) (models.Incident, bool)
}

// Store is a simple in-memory persistence for signals and incidents
type Store struct {
	mu        sync.RWMutex
	signals   map[string]models.Signal
	incidents map[string]models.Incident
}

func NewStore() *Store {
	return &Store{
		signals:   make(map[string]models.Signal),
		incidents: make(map[string]models.Incident),
	}
}

func (s *Store) SaveSignal(sig models.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sig.ID == "" {
		sig.ID = util.GenID()
	}
	if sig.Timestamp.IsZero() {
		sig.Timestamp = time.Now().UTC()
	}
	sig.Payload = RedactPayloadByAllowList(sig.Payload, payloadAllowListSnapshot())
	s.signals[sig.ID] = sig
}

func (s *Store) GetSignal(id string) (models.Signal, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sig, ok := s.signals[id]
	return sig, ok
}

func (s *Store) DeleteSignalsOlderThan(cutoff time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := 0
	for id, sig := range s.signals {
		if !sig.Timestamp.IsZero() && sig.Timestamp.Before(cutoff) {
			delete(s.signals, id)
			deleted++
		}
	}
	return deleted
}

func (s *Store) SaveIncident(inc models.Incident) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if inc.ID == "" {
		inc.ID = util.GenID()
	}
	if inc.CreatedAt.IsZero() {
		inc.CreatedAt = time.Now().UTC()
	}
	inc.UpdatedAt = time.Now().UTC()
	s.incidents[inc.ID] = inc
}

func (s *Store) ListIncidents() []models.Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]models.Incident, 0, len(s.incidents))
	for _, v := range s.incidents {
		list = append(list, v)
	}
	return list
}

func (s *Store) GetIncident(id string) (models.Incident, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inc, ok := s.incidents[id]
	return inc, ok
}
