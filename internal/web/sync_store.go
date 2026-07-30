package web

import (
	"context"
	"sync"
	"time"

	"code.p-fruck.eu/spond-webcal/internal/api"
)

type EventSyncStore interface {
	UpsertUserEvents(ctx context.Context, userID uint, events []api.Event, syncedAt time.Time) error
	LastSyncedAtByUserID(ctx context.Context, userID uint) (*time.Time, error)
}

type MemoryEventSyncStore struct {
	mu           sync.RWMutex
	lastSyncedBy map[uint]time.Time
}

func NewMemoryEventSyncStore() *MemoryEventSyncStore {
	return &MemoryEventSyncStore{lastSyncedBy: map[uint]time.Time{}}
}

func (s *MemoryEventSyncStore) UpsertUserEvents(_ context.Context, userID uint, _ []api.Event, syncedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastSyncedBy[userID] = syncedAt.UTC()
	return nil
}

func (s *MemoryEventSyncStore) LastSyncedAtByUserID(_ context.Context, userID uint) (*time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lastSynced, ok := s.lastSyncedBy[userID]
	if !ok || lastSynced.IsZero() {
		return nil, nil
	}

	value := lastSynced.UTC()
	return &value, nil
}
