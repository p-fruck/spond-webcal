package db

import (
	"context"
	"fmt"
	"strings"

	"code.p-fruck.eu/spond-webcal/internal/caldav"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Open(dbURL string) (*gorm.DB, error) {
	var dialector gorm.Dialector
	if isPostgresURL(dbURL) {
		dialector = postgres.Open(dbURL)
	} else {
		dialector = sqlite.Open(dbURL)
	}

	database, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return database, nil
}

func AutoMigrate(database *gorm.DB) error {
	err := database.AutoMigrate(
		&User{},
		&Calendar{},
		&AccessToken{},
		&SpondCache{},
		&EventResponse{},
		&CalDAVResource{},
	)
	if err != nil {
		return fmt.Errorf("auto-migrate database: %w", err)
	}

	return nil
}

func OpenAndMigrate(dbURL string) (*gorm.DB, error) {
	database, err := Open(dbURL)
	if err != nil {
		return nil, err
	}

	if err := AutoMigrate(database); err != nil {
		return nil, err
	}

	return database, nil
}

func isPostgresURL(dbURL string) bool {
	return strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://")
}

type CalDAVStore struct {
	database *gorm.DB
}

func NewCalDAVStore(database *gorm.DB) *CalDAVStore {
	return &CalDAVStore{database: database}
}

func (s *CalDAVStore) ListResources(ctx context.Context, userKey string) ([]caldav.Resource, error) {
	var rows []CalDAVResource
	err := s.database.WithContext(ctx).
		Where("user_key = ?", userKey).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list caldav resources: %w", err)
	}

	resources := make([]caldav.Resource, 0, len(rows))
	for _, row := range rows {
		resources = append(resources, caldav.Resource{
			Path:    row.ResourcePath,
			Content: row.Content,
		})
	}

	return resources, nil
}

func (s *CalDAVStore) PutResource(ctx context.Context, userKey, resourcePath string, content []byte) error {
	var row CalDAVResource
	err := s.database.WithContext(ctx).
		Where("user_key = ? AND resource_path = ?", userKey, resourcePath).
		First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			newRow := CalDAVResource{
				UserKey:      userKey,
				ResourcePath: resourcePath,
				Content:      content,
			}
			if createErr := s.database.WithContext(ctx).Create(&newRow).Error; createErr != nil {
				return fmt.Errorf("create caldav resource: %w", createErr)
			}
			return nil
		}

		return fmt.Errorf("query caldav resource: %w", err)
	}

	row.Content = content
	if saveErr := s.database.WithContext(ctx).Save(&row).Error; saveErr != nil {
		return fmt.Errorf("update caldav resource: %w", saveErr)
	}

	return nil
}

func (s *CalDAVStore) DeleteResource(ctx context.Context, userKey, resourcePath string) error {
	err := s.database.WithContext(ctx).
		Where("user_key = ? AND resource_path = ?", userKey, resourcePath).
		Delete(&CalDAVResource{}).Error
	if err != nil {
		return fmt.Errorf("delete caldav resource: %w", err)
	}

	return nil
}
