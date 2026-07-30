package db

import (
	"context"
	"testing"
	"time"
)

func TestAccessTokenStoreCreateAndLookup(t *testing.T) {
	database, err := OpenAndMigrate("file:access_tokens_create_lookup?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open and migrate sqlite: %v", err)
	}

	store := NewAccessTokenStore(database)
	expiresAt := time.Now().UTC().Add(2 * time.Hour)
	if err := store.CreateAccessToken(context.Background(), 7, "token-abc", "ical", `{"groups":[]}`, &expiresAt); err != nil {
		t.Fatalf("create access token: %v", err)
	}

	record, err := store.AccessTokenByToken(context.Background(), "token-abc")
	if err != nil {
		t.Fatalf("lookup access token: %v", err)
	}

	if record.UserID != 7 {
		t.Fatalf("expected user id 7, got %d", record.UserID)
	}
	if record.Category != "ical" {
		t.Fatalf("expected category ical, got %q", record.Category)
	}
	if record.Scope != `{"groups":[]}` {
		t.Fatalf("unexpected scope payload %q", record.Scope)
	}
	if record.ExpiresAt == nil || !record.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected expiry %v", record.ExpiresAt)
	}
}

func TestAccessTokenStoreLookupMissingToken(t *testing.T) {
	database, err := OpenAndMigrate("file:access_tokens_lookup_missing?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open and migrate sqlite: %v", err)
	}

	store := NewAccessTokenStore(database)
	if _, err := store.AccessTokenByToken(context.Background(), "missing-token"); err == nil {
		t.Fatal("expected missing token lookup error")
	}
}

func TestAccessTokenStoreListByUserID(t *testing.T) {
	database, err := OpenAndMigrate("file:access_tokens_list_by_user?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open and migrate sqlite: %v", err)
	}

	store := NewAccessTokenStore(database)
	if err := store.CreateAccessToken(context.Background(), 7, "token-1", "ical", `{"groups":[]}`, nil); err != nil {
		t.Fatalf("create first access token: %v", err)
	}
	if err := store.CreateAccessToken(context.Background(), 8, "token-2", "ical", `{"groups":[]}`, nil); err != nil {
		t.Fatalf("create second access token: %v", err)
	}

	records, err := store.ListAccessTokensByUserID(context.Background(), 7)
	if err != nil {
		t.Fatalf("list access tokens: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 access token for user 7, got %d", len(records))
	}

	if records[0].Token != "token-1" {
		t.Fatalf("expected token-1, got %q", records[0].Token)
	}
}

func TestAccessTokenStoreDeleteByID(t *testing.T) {
	database, err := OpenAndMigrate("file:access_tokens_delete_by_id?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open and migrate sqlite: %v", err)
	}

	store := NewAccessTokenStore(database)
	if err := store.CreateAccessToken(context.Background(), 7, "token-delete", "ical", `{"groups":[]}`, nil); err != nil {
		t.Fatalf("create token: %v", err)
	}

	records, err := store.ListAccessTokensByUserID(context.Background(), 7)
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 token, got %d", len(records))
	}

	if err := store.DeleteAccessTokenByID(context.Background(), 7, records[0].ID); err != nil {
		t.Fatalf("delete token by id: %v", err)
	}

	if _, err := store.AccessTokenByToken(context.Background(), "token-delete"); err == nil {
		t.Fatal("expected deleted token lookup to fail")
	}
}

func TestAccessTokenStoreDeleteExpiredByUserID(t *testing.T) {
	database, err := OpenAndMigrate("file:access_tokens_gc?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open and migrate sqlite: %v", err)
	}

	now := time.Now().UTC()
	expired := now.Add(-1 * time.Hour)
	active := now.Add(1 * time.Hour)

	store := NewAccessTokenStore(database)
	if err := store.CreateAccessToken(context.Background(), 7, "token-expired", "ical", `{"groups":[]}`, &expired); err != nil {
		t.Fatalf("create expired token: %v", err)
	}
	if err := store.CreateAccessToken(context.Background(), 7, "token-active", "ical", `{"groups":[]}`, &active); err != nil {
		t.Fatalf("create active token: %v", err)
	}

	deleted, err := store.DeleteExpiredAccessTokensByUserID(context.Background(), 7, now)
	if err != nil {
		t.Fatalf("delete expired tokens: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted token, got %d", deleted)
	}

	if _, err := store.AccessTokenByToken(context.Background(), "token-expired"); err == nil {
		t.Fatal("expected expired token to be deleted")
	}

	if _, err := store.AccessTokenByToken(context.Background(), "token-active"); err != nil {
		t.Fatalf("expected active token to remain: %v", err)
	}
}
