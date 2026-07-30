package db

import (
	"context"
	"testing"
	"time"
)

func TestUserTokenStoreUpsertAndReadBack(t *testing.T) {
	database, err := OpenAndMigrate("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open and migrate: %v", err)
	}

	store := NewUserTokenStore(database)
	expiresAt := time.Now().UTC().Add(30 * time.Minute)
	refreshExpiresAt := time.Now().UTC().Add(24 * time.Hour)

	userID, err := store.UpsertUserToken(context.Background(), "P1", "ada@example.com", "TOKEN1", &expiresAt, "REFRESH1", &refreshExpiresAt)
	if err != nil {
		t.Fatalf("upsert token: %v", err)
	}

	token, err := store.TokenByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("read token by user id: %v", err)
	}

	if token != "TOKEN1" {
		t.Fatalf("expected TOKEN1, got %q", token)
	}

	updatedID, err := store.UpsertUserToken(context.Background(), "P1", "ada@example.com", "TOKEN2", &expiresAt, "REFRESH2", &refreshExpiresAt)
	if err != nil {
		t.Fatalf("update token: %v", err)
	}

	if updatedID != userID {
		t.Fatalf("expected same user id on upsert, got %d vs %d", updatedID, userID)
	}

	token, err = store.TokenByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("read updated token: %v", err)
	}

	if token != "TOKEN2" {
		t.Fatalf("expected TOKEN2, got %q", token)
	}

	active, err := store.ActiveUsersWithToken(context.Background())
	if err != nil {
		t.Fatalf("active users: %v", err)
	}

	if len(active) != 1 {
		t.Fatalf("expected 1 active user, got %d", len(active))
	}

	if active[0].TokenExpires == nil {
		t.Fatal("expected token expiry to be stored")
	}

	if active[0].RefreshToken != "REFRESH2" {
		t.Fatalf("expected updated refresh token REFRESH2, got %q", active[0].RefreshToken)
	}
}
