package service

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/storage"
)

func TestDeleteCompany(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-company-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	store, err := db.Open(filepath.Join(tempDir, "test.db"))
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

	org, err := svc.CreateOrganization(ctx, "Test Org")
	if err != nil {
		t.Fatal(err)
	}

	code := "123456789"
	if err := svc.UpsertCompany(ctx, domain.Company{
		OrgID: org.ID,
		Title: "Unused Co",
		Code:  &code,
	}); err != nil {
		t.Fatal(err)
	}

	companies, err := svc.ListCompanies(ctx, org.ID, CompanyListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(companies) != 1 {
		t.Fatalf("expected 1 company, got %d", len(companies))
	}
	companyID := companies[0].ID

	if err := svc.DeleteCompany(ctx, org.ID, companyID); err != nil {
		t.Fatalf("DeleteCompany: %v", err)
	}
	if _, err := svc.GetCompany(ctx, companyID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected company to be deleted, got err=%v", err)
	}

	linkedCode := "987654321"
	if err := svc.UpsertCompany(ctx, domain.Company{
		OrgID: org.ID,
		Title: "Linked Co",
		Code:  &linkedCode,
	}); err != nil {
		t.Fatal(err)
	}
	companies, err = svc.ListCompanies(ctx, org.ID, CompanyListParams{})
	if err != nil {
		t.Fatal(err)
	}
	linkedID := companies[0].ID

	user, err := svc.CreateUser(ctx, "user@test.com", "password1", "User")
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().Unix()
	_, err = store.DB().Exec(`
		INSERT INTO invoices (id, user_id, org_id, status, filename, checksum, storage_path, seller_code, seller_name, created_at, updated_at)
		VALUES (?, ?, ?, 'processed', 'inv.pdf', ?, 'path/inv.pdf', ?, 'Linked Co', ?, ?)`,
		uuid.New().String(), user.ID, org.ID, uuid.New().String(), linkedCode, now, now)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteCompany(ctx, org.ID, linkedID); !errors.Is(err, ErrCompanyHasInvoices) {
		t.Fatalf("DeleteCompany linked = %v, want ErrCompanyHasInvoices", err)
	}
}
