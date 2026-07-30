package web

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"code.p-fruck.eu/spond-webcal/internal/db"
)

type AccessTokenStore interface {
	CreateAccessToken(ctx context.Context, userID uint, token, category, scope string, expiresAt *time.Time) error
	AccessTokenByToken(ctx context.Context, token string) (db.AccessTokenRecord, error)
	ListAccessTokensByUserID(ctx context.Context, userID uint) ([]db.AccessTokenRecord, error)
	DeleteAccessTokenByID(ctx context.Context, userID, tokenID uint) error
	DeleteAccessTokenByToken(ctx context.Context, token string) error
	DeleteExpiredAccessTokensByUserID(ctx context.Context, userID uint, now time.Time) (int, error)
}

type MemoryAccessTokenStore struct {
	mu      sync.RWMutex
	nextID  uint
	byToken map[string]memoryAccessToken
}

type memoryAccessToken struct {
	id        uint
	userID    uint
	category  string
	scope     string
	expiresAt *time.Time
	createdAt time.Time
}

func NewMemoryAccessTokenStore() *MemoryAccessTokenStore {
	return &MemoryAccessTokenStore{nextID: 1, byToken: map[string]memoryAccessToken{}}
}

func (s *MemoryAccessTokenStore) CreateAccessToken(_ context.Context, userID uint, token, category, scope string, expiresAt *time.Time) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is required")
	}
	if userID == 0 {
		return fmt.Errorf("user id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID
	s.nextID++
	s.byToken[token] = memoryAccessToken{
		id:        id,
		userID:    userID,
		category:  strings.TrimSpace(strings.ToLower(category)),
		scope:     scope,
		expiresAt: expiresAt,
		createdAt: time.Now().UTC(),
	}
	return nil
}

func (s *MemoryAccessTokenStore) AccessTokenByToken(_ context.Context, token string) (db.AccessTokenRecord, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return db.AccessTokenRecord{}, fmt.Errorf("token is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.byToken[token]
	if !ok {
		return db.AccessTokenRecord{}, fmt.Errorf("access token not found")
	}

	return db.AccessTokenRecord{
		ID:        record.id,
		UserID:    record.userID,
		Token:     token,
		Category:  record.category,
		Scope:     record.scope,
		ExpiresAt: record.expiresAt,
		CreatedAt: record.createdAt,
	}, nil
}

func (s *MemoryAccessTokenStore) ListAccessTokensByUserID(_ context.Context, userID uint) ([]db.AccessTokenRecord, error) {
	if userID == 0 {
		return nil, fmt.Errorf("user id is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	records := make([]db.AccessTokenRecord, 0, len(s.byToken))
	for token, entry := range s.byToken {
		if entry.userID != userID {
			continue
		}

		records = append(records, db.AccessTokenRecord{
			ID:        entry.id,
			UserID:    entry.userID,
			Token:     token,
			Category:  entry.category,
			Scope:     entry.scope,
			ExpiresAt: entry.expiresAt,
			CreatedAt: entry.createdAt,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})

	return records, nil
}

func (s *MemoryAccessTokenStore) DeleteAccessTokenByID(_ context.Context, userID, tokenID uint) error {
	if userID == 0 {
		return fmt.Errorf("user id is required")
	}
	if tokenID == 0 {
		return fmt.Errorf("token id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for token, entry := range s.byToken {
		if entry.id == tokenID && entry.userID == userID {
			delete(s.byToken, token)
			return nil
		}
	}

	return fmt.Errorf("access token not found")
}

func (s *MemoryAccessTokenStore) DeleteAccessTokenByToken(_ context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byToken[token]; !ok {
		return fmt.Errorf("access token not found")
	}

	delete(s.byToken, token)
	return nil
}

func (s *MemoryAccessTokenStore) DeleteExpiredAccessTokensByUserID(_ context.Context, userID uint, now time.Time) (int, error) {
	if userID == 0 {
		return 0, fmt.Errorf("user id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	deleted := 0
	for token, entry := range s.byToken {
		if entry.userID != userID {
			continue
		}
		if entry.expiresAt == nil || entry.expiresAt.After(now.UTC()) {
			continue
		}

		delete(s.byToken, token)
		deleted++
	}

	return deleted, nil
}
