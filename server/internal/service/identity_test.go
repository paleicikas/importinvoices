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
	urls := map[string]string{"event": "http://example.com"}
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
