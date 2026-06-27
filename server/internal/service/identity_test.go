package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/storage"
)

func TestIdentity(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-db-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	dbPath := filepath.Join(tempDir, "test.db")
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	strg, _ := storage.New(filepath.Join(tempDir, "storage"))
	svc := New(store, strg, nil)

	ctx := context.Background()

	user, err := svc.CreateUser(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}

	// Test Authenticate
	authUser, err := svc.Authenticate(ctx, "test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to authenticate: %v", err)
	}
	if authUser.ID != user.ID {
		t.Errorf("expected ID %s, got %s", user.ID, authUser.ID)
	}

	_, err = svc.CreateUser(ctx, "short@example.com", "short", "Short User")
	if err != ErrPasswordTooShort {
		t.Fatalf("expected ErrPasswordTooShort, got %v", err)
	}

	expiredSessionID := "expired-session"
	expiredToken := "expired-token"
	_, err = store.DB().ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, token, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		expiredSessionID, user.ID, expiredToken, time.Now().Add(-time.Hour).Unix(), time.Now().Unix())
	if err != nil {
		t.Fatalf("insert expired session: %v", err)
	}
	if err := svc.CleanupExpiredSessions(ctx); err != nil {
		t.Fatalf("CleanupExpiredSessions: %v", err)
	}
	var expiredCount int
	if err := store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE token = ?", expiredToken).Scan(&expiredCount); err != nil {
		t.Fatal(err)
	}
	if expiredCount != 0 {
		t.Fatal("expected expired session to be removed")
	}

	session, err := svc.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := svc.GetUserBySessionToken(ctx, session.Token); err != nil {
		t.Fatalf("GetUserBySessionToken: %v", err)
	}
	if err := svc.UpdatePassword(ctx, user.ID, "newpassword1"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	if _, err := svc.GetUserBySessionToken(ctx, session.Token); err == nil {
		t.Fatal("expected old session to be invalid after password change")
	}

	// Test DeleteUser
	err = svc.DeleteUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to delete user: %v", err)
	}
	_, err = svc.GetUser(ctx, user.ID)
	if err == nil {
		t.Error("expected error getting deleted user, got nil")
	}
}
