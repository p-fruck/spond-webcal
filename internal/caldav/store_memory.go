package caldav

import (
	"context"
	"sync"
)

type MemoryResourceStore struct {
	mu   sync.RWMutex
	data map[string]map[string][]byte
}

func NewMemoryResourceStore() *MemoryResourceStore {
	return &MemoryResourceStore{data: make(map[string]map[string][]byte)}
}

func (s *MemoryResourceStore) ListResources(_ context.Context, userKey string) ([]Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userData := s.data[userKey]
	resources := make([]Resource, 0, len(userData))
	for p, content := range userData {
		copied := make([]byte, len(content))
		copy(copied, content)
		resources = append(resources, Resource{Path: p, Content: copied})
	}

	return resources, nil
}

func (s *MemoryResourceStore) PutResource(_ context.Context, userKey, resourcePath string, content []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[userKey]; !ok {
		s.data[userKey] = make(map[string][]byte)
	}

	copied := make([]byte, len(content))
	copy(copied, content)
	s.data[userKey][resourcePath] = copied

	return nil
}

func (s *MemoryResourceStore) DeleteResource(_ context.Context, userKey, resourcePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[userKey]; ok {
		delete(s.data[userKey], resourcePath)
	}

	return nil
}
