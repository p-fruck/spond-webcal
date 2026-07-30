package db

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type UserTokenStore struct {
	database *gorm.DB
}

func NewUserTokenStore(database *gorm.DB) *UserTokenStore {
	return &UserTokenStore{database: database}
}

func (s *UserTokenStore) UpsertUserToken(ctx context.Context, profileID, email, token string) (uint, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return 0, fmt.Errorf("profile id is required")
	}

	var user User
	err := s.database.WithContext(ctx).Where("profile_id = ?", profileID).First(&user).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return 0, fmt.Errorf("query user by profile id: %w", err)
		}

		newUser := User{
			ProfileID:  profileID,
			SpondEmail: strings.TrimSpace(email),
			SpondToken: strings.TrimSpace(token),
		}
		if createErr := s.database.WithContext(ctx).Create(&newUser).Error; createErr != nil {
			return 0, fmt.Errorf("create user: %w", createErr)
		}

		return newUser.ID, nil
	}

	user.SpondEmail = strings.TrimSpace(email)
	user.SpondToken = strings.TrimSpace(token)
	if saveErr := s.database.WithContext(ctx).Save(&user).Error; saveErr != nil {
		return 0, fmt.Errorf("update user token: %w", saveErr)
	}

	return user.ID, nil
}

func (s *UserTokenStore) TokenByUserID(ctx context.Context, userID uint) (string, error) {
	var user User
	err := s.database.WithContext(ctx).First(&user, userID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("user %d not found", userID)
		}

		return "", fmt.Errorf("query user by id: %w", err)
	}

	token := strings.TrimSpace(user.SpondToken)
	if token == "" {
		return "", fmt.Errorf("user %d has empty token", userID)
	}

	return token, nil
}
