package db

import (
	"fmt"
	"strings"

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
