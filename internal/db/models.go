package db

import "time"

type User struct {
	ID                  uint   `gorm:"primaryKey"`
	ProfileID           string `gorm:"uniqueIndex;size:128"`
	SpondEmail          string
	SpondToken          string
	TokenExpires        *time.Time
	RefreshToken        string
	RefreshTokenExpires *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Calendar struct {
	ID         uint `gorm:"primaryKey"`
	UserID     uint
	Name       string
	Visibility string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type AccessToken struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"index"`
	Token     string `gorm:"uniqueIndex;size:256"`
	Category  string `gorm:"size:64"`
	Scope     string
	ExpiresAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SpondCache struct {
	ID         uint `gorm:"primaryKey"`
	UserID     uint
	EventID    string
	Payload    []byte
	LastSynced time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type EventResponse struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint
	EventID   string
	Response  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CalDAVResource struct {
	ID           uint   `gorm:"primaryKey"`
	UserKey      string `gorm:"index:idx_caldav_user_path,priority:1;size:128;not null"`
	ResourcePath string `gorm:"index:idx_caldav_user_path,priority:2;size:512;not null"`
	Content      []byte
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
