package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/media"
	"github.com/paleicikas/importinvoices/server/internal/storage"
	"github.com/paleicikas/importinvoices/server/internal/testutil"
)

func NewTestService(t *testing.T) (*Service, *db.Store, *storage.Storage, *media.MediaService) {
	t.Helper()
	dir := t.TempDir()
	store := testutil.NewTestDB(t)

	strg, err := storage.New(filepath.Join(dir, "files"))
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	mediaSvc := media.New(filepath.Join(dir, "temp"))
	svc := New(store, strg, mediaSvc)
	return svc, store, strg, mediaSvc
}

func SetupUser(t *testing.T, svc *Service) string {
	t.Helper()
	ctx := context.Background()
	err := svc.Setup(ctx, "Test Org", "Admin", "admin@test.com", "secret123")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	return "admin@test.com"
}

func strPtr(s string) *string { return &s }
func floatPtr(f float64) *float64 { return &f }
func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int { return &i }
