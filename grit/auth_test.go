package grit

import (
	"os"
	"testing"
)

func TestAuth_Revocation(t *testing.T) {
	// Clean up any existing auth.db for a fresh test
	os.Remove("auth.db")
	defer os.Remove("auth.db")

	if err := InitSQLite(); err != nil {
		t.Fatalf("failed to init sqlite: %v", err)
	}

	token := "test-token-123"

	// 1. Initially not revoked
	if sqliteBlacklistContains(token) {
		t.Error("token should not be revoked initially")
	}

	// 2. Add to revocation list
	sqliteBlacklistAdd(token)

	// 3. Verify it is revoked
	if !sqliteBlacklistContains(token) {
		t.Error("token should be revoked after adding")
	}

	// 4. Reset DB connection (simulate server restart)
	sqliteDB = nil
	if !sqliteBlacklistContains(token) {
		t.Error("token should STILL be revoked after 'restart' (persistence test)")
	}
}
