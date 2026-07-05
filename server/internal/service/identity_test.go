package service

import (
	"context"
	"strings"
	"testing"
)

func TestIdentity(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	ctx := context.Background()

	// 1. Create User
	user, err := svc.CreateUser(ctx, "test@example.com", "password123", "Test User")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}

	// 2. Authenticate
	gotUser, err := svc.Authenticate(ctx, "test@example.com", "password123")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if gotUser.ID != user.ID {
		t.Errorf("expected ID %s, got %s", user.ID, gotUser.ID)
	}

	// 3. Authenticate Fail
	_, err = svc.Authenticate(ctx, "test@example.com", "wrong")
	if err == nil {
		t.Error("expected authentication failure")
	}

	// 4. GetUserByEmail
	gotByEmail, err := svc.GetUserByEmail(ctx, "Test@Example.com") // mixed-case on purpose
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if gotByEmail.ID != user.ID {
		t.Errorf("GetUserByEmail ID = %s, want %s", gotByEmail.ID, user.ID)
	}
	if _, err := svc.GetUserByEmail(ctx, "nope@example.com"); err == nil {
		t.Error("GetUserByEmail: expected error for unknown email, got nil")
	}
}

func TestUserPasswordAndWebhooks(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()
	user, _ := svc.Authenticate(ctx, "admin@test.com", "secret123")

	// 1. Update Password
	err := svc.UpdatePassword(ctx, user.ID, "new-password123")
	if err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	_, err = svc.Authenticate(ctx, "admin@test.com", "secret123")
	if err == nil {
		t.Error("expected authentication failure with old password")
	}
	_, err = svc.Authenticate(ctx, "admin@test.com", "new-password123")
	if err != nil {
		t.Errorf("expected authentication success with new password: %v", err)
	}

	// 2. Update Webhooks
	urls := map[string]string{"event": "https://example.com"}
	err = svc.UpdateUserWebhooks(ctx, user.ID, urls)
	if err != nil {
		t.Fatalf("UpdateUserWebhooks: %v", err)
	}
	gotUser, _ := svc.GetUser(ctx, user.ID)
	if gotUser.WebhookUrls == nil || !strings.Contains(*gotUser.WebhookUrls, "example.com") {
		t.Errorf("WebhookUrls mismatch: %v", gotUser.WebhookUrls)
	}

	// 3. Update User
	err = svc.UpdateUser(ctx, user.ID, "New Name", "new@test.com")
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	gotUser, _ = svc.GetUser(ctx, user.ID)
	if gotUser.Name != "New Name" || gotUser.Email != "new@test.com" {
		t.Errorf("UpdateUser mismatch")
	}

	// 4. Delete User
	err = svc.DeleteUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	_, err = svc.GetUser(ctx, user.ID)
	if err == nil {
		t.Error("expected error getting deleted user")
	}
}

func TestOrganization(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	ctx := context.Background()

	_, err := svc.CreateOrganization(ctx, "Test Org")
	if err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}

	gotOrg, err := svc.GetOrganization(ctx)
	if err != nil {
		t.Fatalf("GetOrganization: %v", err)
	}
	if gotOrg.Title != "Test Org" {
		t.Errorf("expected title Test Org, got %s", gotOrg.Title)
	}

	err = svc.UpdateOrganization(ctx, "New Title")
	if err != nil {
		t.Fatalf("UpdateOrganization: %v", err)
	}
	gotOrg, _ = svc.GetOrganization(ctx)
	if gotOrg.Title != "New Title" {
		t.Errorf("expected title New Title, got %s", gotOrg.Title)
	}
}

// TestUpdateUserWebhooks_RejectsInvalid verifies P3-7: webhook URLs are
// validated at SAVE time (not only at send time), so an admin cannot persist
// an internal/non-HTTPS URL. Each bad URL must be rejected; a valid HTTPS
// public URL must still be accepted.
func TestUpdateUserWebhooks_RejectsInvalid(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()
	user, _ := svc.Authenticate(ctx, "admin@test.com", "secret123")

	bad := []string{
		"http://public.example.com/hook",  // must be https
		"https://localhost/hook",          // loopback
		"https://127.0.0.1/hook",          // loopback
		"https://169.254.169.254/latest",  // cloud metadata
		"https://metadata.google.invalid", // suspicious host (resolved internal)
		"not-a-url",
	}
	for _, u := range bad {
		err := svc.UpdateUserWebhooks(ctx, user.ID, map[string]string{"invoice.exported": u})
		if err == nil {
			t.Errorf("UpdateUserWebhooks(%q): expected rejection, got nil", u)
		}
	}

	// Valid HTTPS public URL is accepted.
	if err := svc.UpdateUserWebhooks(ctx, user.ID, map[string]string{"invoice.exported": "https://example.com/hook"}); err != nil {
		t.Errorf("UpdateUserWebhooks(valid): unexpected error: %v", err)
	}

	// An empty map clears webhooks without error (no URLs to validate).
	if err := svc.UpdateUserWebhooks(ctx, user.ID, map[string]string{}); err != nil {
		t.Errorf("UpdateUserWebhooks(empty): unexpected error: %v", err)
	}
}
