package web

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type UserTokenStore interface {
	UpsertUserToken(ctx context.Context, profileID, email, token string, tokenExpires *time.Time, refreshToken string, refreshTokenExpires *time.Time) (uint, error)
	TokenByUserID(ctx context.Context, userID uint) (string, error)
	TokenMetadataByUserID(ctx context.Context, userID uint) (string, *time.Time, string, *time.Time, error)
	PromoteRefreshTokenByUserID(ctx context.Context, userID uint) error
}

type MemoryUserTokenStore struct {
	mu     sync.RWMutex
	nextID uint
	users  map[uint]memoryUser
	byProf map[string]uint
}

type memoryUser struct {
	profileID        string
	email            string
	token            string
	expiresAt        *time.Time
	refreshToken     string
	refreshExpiresAt *time.Time
}

func NewMemoryUserTokenStore() *MemoryUserTokenStore {
	return &MemoryUserTokenStore{
		nextID: 1,
		users:  map[uint]memoryUser{},
		byProf: map[string]uint{},
	}
}

func (s *MemoryUserTokenStore) UpsertUserToken(_ context.Context, profileID, email, token string, tokenExpires *time.Time, refreshToken string, refreshTokenExpires *time.Time) (uint, error) {
	if profileID == "" {
		return 0, fmt.Errorf("profile id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if userID, ok := s.byProf[profileID]; ok {
		u := s.users[userID]
		u.email = email
		u.token = token
		u.expiresAt = tokenExpires
		u.refreshToken = refreshToken
		u.refreshExpiresAt = refreshTokenExpires
		s.users[userID] = u
		return userID, nil
	}

	userID := s.nextID
	s.nextID++
	s.byProf[profileID] = userID
	s.users[userID] = memoryUser{profileID: profileID, email: email, token: token, expiresAt: tokenExpires, refreshToken: refreshToken, refreshExpiresAt: refreshTokenExpires}

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

func (s *MemoryUserTokenStore) TokenMetadataByUserID(_ context.Context, userID uint) (string, *time.Time, string, *time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.users[userID]
	if !ok {
		return "", nil, "", nil, fmt.Errorf("user not found")
	}

	return u.token, u.expiresAt, u.refreshToken, u.refreshExpiresAt, nil
}

func (s *MemoryUserTokenStore) PromoteRefreshTokenByUserID(_ context.Context, userID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.users[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}

	if u.refreshToken == "" {
		return fmt.Errorf("refresh token is empty")
	}

	u.token = u.refreshToken
	u.expiresAt = u.refreshExpiresAt
	s.users[userID] = u
	return nil
}
