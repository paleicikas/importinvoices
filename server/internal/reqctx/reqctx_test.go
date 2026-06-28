package reqctx

import (
	"context"
	"testing"

	"github.com/paleicikas/importinvoices/server/internal/domain"
)

func TestUser(t *testing.T) {
	ctx := context.Background()
	u := &domain.User{ID: "123"}

	ctx = WithUser(ctx, u)
	got, ok := User(ctx)
	if !ok {
		t.Fatal("expected user in context")
	}
	if got.ID != "123" {
		t.Errorf("got ID %s, want 123", got.ID)
	}

	_, ok = User(context.Background())
	if ok {
		t.Fatal("expected no user in empty context")
	}
}

func TestOrganization(t *testing.T) {
	ctx := context.Background()
	org := &domain.Organization{ID: "org-123"}

	ctx = WithOrganization(ctx, org)
	got, ok := Organization(ctx)
	if !ok {
		t.Fatal("expected organization in context")
	}
	if got.ID != "org-123" {
		t.Errorf("got ID %s, want org-123", got.ID)
	}

	_, ok = Organization(context.Background())
	if ok {
		t.Fatal("expected no organization in empty context")
	}
}

func TestCSRFToken(t *testing.T) {
	ctx := context.Background()
	token := "test-token"

	ctx = WithCSRFToken(ctx, token)
	got, ok := CSRFToken(ctx)
	if !ok {
		t.Fatal("expected CSRF token in context")
	}
	if got != "test-token" {
		t.Errorf("got token %s, want test-token", got)
	}

	_, ok = CSRFToken(context.Background())
	if ok {
		t.Fatal("expected no CSRF token in empty context")
	}
}
