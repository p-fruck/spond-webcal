package db

import "testing"

func TestIsPostgresURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "postgres scheme", url: "postgres://user:pass@localhost/db", want: true},
		{name: "postgresql scheme", url: "postgresql://user:pass@localhost/db", want: true},
		{name: "sqlite file url", url: "file:./app.db", want: false},
		{name: "sqlite memory", url: "file::memory:?cache=shared", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPostgresURL(tt.url)
			if got != tt.want {
				t.Fatalf("isPostgresURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestOpenAndMigrateSQLiteInMemory(t *testing.T) {
	database, err := OpenAndMigrate("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open and migrate sqlite: %v", err)
	}

	if !database.Migrator().HasTable(&User{}) {
		t.Fatal("expected users table to exist after migration")
	}

	if !database.Migrator().HasTable(&Calendar{}) {
		t.Fatal("expected calendars table to exist after migration")
	}

	if !database.Migrator().HasTable(&AccessToken{}) {
		t.Fatal("expected access_tokens table to exist after migration")
	}

	if !database.Migrator().HasTable(&SpondCache{}) {
		t.Fatal("expected spond_caches table to exist after migration")
	}

	if !database.Migrator().HasTable(&EventResponse{}) {
		t.Fatal("expected event_responses table to exist after migration")
	}
}
