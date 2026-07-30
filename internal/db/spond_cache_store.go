package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"code.p-fruck.eu/spond-webcal/internal/api"

	"gorm.io/gorm"
)

type SpondCacheStore struct {
	database *gorm.DB
}

func NewSpondCacheStore(database *gorm.DB) *SpondCacheStore {
	return &SpondCacheStore{database: database}
}

func (s *SpondCacheStore) UpsertUserEvents(ctx context.Context, userID uint, events []api.Event, syncedAt time.Time) error {
	for _, event := range events {
		eventID := strings.TrimSpace(event.Id)
		if eventID == "" {
			continue
		}

		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal event %q payload: %w", eventID, err)
		}

		var row SpondCache
		queryErr := s.database.WithContext(ctx).
			Where("user_id = ? AND event_id = ?", userID, eventID).
			First(&row).Error
		if queryErr != nil {
			if queryErr == gorm.ErrRecordNotFound {
				newRow := SpondCache{
					UserID:     userID,
					EventID:    eventID,
					Payload:    payload,
					LastSynced: syncedAt,
				}
				if createErr := s.database.WithContext(ctx).Create(&newRow).Error; createErr != nil {
					return fmt.Errorf("create spond cache row for event %q: %w", eventID, createErr)
				}
				continue
			}

			return fmt.Errorf("query spond cache row for event %q: %w", eventID, queryErr)
		}

		row.Payload = payload
		row.LastSynced = syncedAt
		if saveErr := s.database.WithContext(ctx).Save(&row).Error; saveErr != nil {
			return fmt.Errorf("update spond cache row for event %q: %w", eventID, saveErr)
		}
	}

	return nil
}

func (s *SpondCacheStore) LastSyncedAtByUserID(ctx context.Context, userID uint) (*time.Time, error) {
	var row SpondCache
	err := s.database.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("last_synced DESC").
		Limit(1).
		First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}

		return nil, fmt.Errorf("query latest spond cache row: %w", err)
	}

	if row.LastSynced.IsZero() {
		return nil, nil
	}

	lastSynced := row.LastSynced.UTC()
	return &lastSynced, nil
}
