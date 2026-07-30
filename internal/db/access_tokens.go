package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type AccessTokenStore struct {
	database *gorm.DB
}

type AccessTokenRecord struct {
	ID        uint
	UserID    uint
	Token     string
	Category  string
	Scope     string
	ExpiresAt *time.Time
	CreatedAt time.Time
}

func NewAccessTokenStore(database *gorm.DB) *AccessTokenStore {
	return &AccessTokenStore{database: database}
}

func (s *AccessTokenStore) CreateAccessToken(ctx context.Context, userID uint, token, category, scope string, expiresAt *time.Time) error {
	if userID == 0 {
		return fmt.Errorf("user id is required")
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is required")
	}

	row := AccessToken{
		UserID:    userID,
		Token:     token,
		Category:  strings.TrimSpace(strings.ToLower(category)),
		Scope:     scope,
		ExpiresAt: expiresAt,
	}

	if err := s.database.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("create access token: %w", err)
	}

	return nil
}

func (s *AccessTokenStore) AccessTokenByToken(ctx context.Context, token string) (AccessTokenRecord, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return AccessTokenRecord{}, fmt.Errorf("token is required")
	}

	var row AccessToken
	if err := s.database.WithContext(ctx).Where("token = ?", token).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return AccessTokenRecord{}, fmt.Errorf("access token not found")
		}

		return AccessTokenRecord{}, fmt.Errorf("query access token: %w", err)
	}

	return AccessTokenRecord{
		ID:        row.ID,
		UserID:    row.UserID,
		Token:     row.Token,
		Category:  strings.TrimSpace(strings.ToLower(row.Category)),
		Scope:     row.Scope,
		ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (s *AccessTokenStore) ListAccessTokensByUserID(ctx context.Context, userID uint) ([]AccessTokenRecord, error) {
	if userID == 0 {
		return nil, fmt.Errorf("user id is required")
	}

	var rows []AccessToken
	if err := s.database.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list access tokens: %w", err)
	}

	records := make([]AccessTokenRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, AccessTokenRecord{
			ID:        row.ID,
			UserID:    row.UserID,
			Token:     row.Token,
			Category:  strings.TrimSpace(strings.ToLower(row.Category)),
			Scope:     row.Scope,
			ExpiresAt: row.ExpiresAt,
			CreatedAt: row.CreatedAt,
		})
	}

	return records, nil
}

func (s *AccessTokenStore) DeleteAccessTokenByID(ctx context.Context, userID, tokenID uint) error {
	if userID == 0 {
		return fmt.Errorf("user id is required")
	}
	if tokenID == 0 {
		return fmt.Errorf("token id is required")
	}

	result := s.database.WithContext(ctx).
		Where("id = ? AND user_id = ?", tokenID, userID).
		Delete(&AccessToken{})
	if result.Error != nil {
		return fmt.Errorf("delete access token by id: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("access token not found")
	}

	return nil
}

func (s *AccessTokenStore) DeleteAccessTokenByToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is required")
	}

	result := s.database.WithContext(ctx).
		Where("token = ?", token).
		Delete(&AccessToken{})
	if result.Error != nil {
		return fmt.Errorf("delete access token by token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("access token not found")
	}

	return nil
}

func (s *AccessTokenStore) DeleteExpiredAccessTokensByUserID(ctx context.Context, userID uint, now time.Time) (int, error) {
	if userID == 0 {
		return 0, fmt.Errorf("user id is required")
	}

	result := s.database.WithContext(ctx).
		Where("user_id = ? AND expires_at IS NOT NULL AND expires_at <= ?", userID, now.UTC()).
		Delete(&AccessToken{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete expired access tokens by user id: %w", result.Error)
	}

	return int(result.RowsAffected), nil
}
