package web

import (
	"context"
	"fmt"
	"sync"
)

type UserTokenStore interface {
	UpsertUserToken(ctx context.Context, profileID, email, token string) (uint, error)
	TokenByUserID(ctx context.Context, userID uint) (string, error)
}

type MemoryUserTokenStore struct {
	mu     sync.RWMutex
	nextID uint
	users  map[uint]memoryUser
	byProf map[string]uint
}

type memoryUser struct {
	profileID string
	email     string
	token     string
}

func NewMemoryUserTokenStore() *MemoryUserTokenStore {
	return &MemoryUserTokenStore{
		nextID: 1,
		users:  map[uint]memoryUser{},
		byProf: map[string]uint{},
	}
}

func (s *MemoryUserTokenStore) UpsertUserToken(_ context.Context, profileID, email, token string) (uint, error) {
	if profileID == "" {
		return 0, fmt.Errorf("profile id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if userID, ok := s.byProf[profileID]; ok {
		u := s.users[userID]
		u.email = email
		u.token = token
		s.users[userID] = u
		return userID, nil
	}

	userID := s.nextID
	s.nextID++
	s.byProf[profileID] = userID
	s.users[userID] = memoryUser{profileID: profileID, email: email, token: token}

	return userID, nil
}

func (s *MemoryUserTokenStore) TokenByUserID(_ context.Context, userID uint) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.users[userID]
	if !ok {
		return "", fmt.Errorf("user not found")
	}

	if u.token == "" {
		return "", fmt.Errorf("user token is empty")
	}

	return u.token, nil
}
