package db

import "time"

type User struct {
	ID           uint `gorm:"primaryKey"`
	SpondEmail   string
	SpondToken   string
	TokenExpires *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
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
	ID         uint `gorm:"primaryKey"`
	CalendarID uint
	Token      string
	Scope      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
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
