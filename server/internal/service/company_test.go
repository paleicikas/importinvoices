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
	if _, err := svc.UpsertCompany(ctx, domain.Company{
		OrgID: org.ID,
		Title: "Unused Co",
		Code:  &code,
	}, nil); err != nil {
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
	if _, err := svc.UpsertCompany(ctx, domain.Company{
		OrgID: org.ID,
		Title: "Linked Co",
		Code:  &linkedCode,
	}, nil); err != nil {
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
		INSERT INTO invoices (id, user_id, org_id, status, filename, checksum, storage_path, seller_code, seller_name, seller_company_id, created_at, updated_at)
		VALUES (?, ?, ?, 'processed', 'inv.pdf', ?, 'path/inv.pdf', ?, 'Linked Co', ?, ?, ?)`,
		uuid.New().String(), user.ID, org.ID, uuid.New().String(), linkedCode, linkedID, now, now)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteCompany(ctx, org.ID, linkedID); !errors.Is(err, ErrCompanyHasInvoices) {
		t.Fatalf("DeleteCompany linked = %v, want ErrCompanyHasInvoices", err)
	}
}

func TestSearchCompaniesForMerge(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-company-search-*")
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

	code := "C001"
	vat := "LT111"
	if _, err := svc.UpsertCompany(ctx, domain.Company{OrgID: org.ID, Title: "Alpha Co", Code: &code, VATCode: &vat}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpsertCompany(ctx, domain.Company{OrgID: org.ID, Title: "Beta Co"}, nil); err != nil {
		t.Fatal(err)
	}
	// Company in a different org must never appear in results.
	otherOrg, err := svc.CreateOrganization(ctx, "Other Org")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpsertCompany(ctx, domain.Company{OrgID: otherOrg.ID, Title: "Alpha Other Org"}, nil); err != nil {
		t.Fatal(err)
	}

	all, err := svc.ListCompanies(ctx, org.ID, CompanyListParams{})
	if err != nil {
		t.Fatal(err)
	}
	var alphaID string
	for _, c := range all {
		if c.Title == "Alpha Co" {
			alphaID = c.ID
		}
	}
	if alphaID == "" {
		t.Fatal("Alpha Co not created")
	}

	// Title search returns Alpha, not the cross-org namesake.
	hits, err := svc.SearchCompaniesForMerge(ctx, org.ID, alphaID, "Alpha", 20)
	if err != nil {
		t.Fatalf("SearchCompaniesForMerge: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected exclude=alphaID to hide Alpha, got %d hits", len(hits))
	}

	hits, err = svc.SearchCompaniesForMerge(ctx, org.ID, "", "Alpha", 20)
	if err != nil {
		t.Fatalf("SearchCompaniesForMerge: %v", err)
	}
	if len(hits) != 1 || hits[0].Title != "Alpha Co" {
		t.Fatalf("expected only Alpha Co, got %+v", hits)
	}

	// VAT search uses the normalized form (LT prefix stripped to "111").
	hits, err = svc.SearchCompaniesForMerge(ctx, org.ID, "", "111", 20)
	if err != nil {
		t.Fatalf("SearchCompaniesForMerge by vat: %v", err)
	}
	if len(hits) != 1 || hits[0].Title != "Alpha Co" {
		t.Fatalf("expected Alpha by VAT, got %+v", hits)
	}

	// Code search.
	hits, err = svc.SearchCompaniesForMerge(ctx, org.ID, "", "C001", 20)
	if err != nil {
		t.Fatalf("SearchCompaniesForMerge by code: %v", err)
	}
	if len(hits) != 1 || hits[0].Title != "Alpha Co" {
		t.Fatalf("expected Alpha by code, got %+v", hits)
	}

	// Empty search returns all org companies (capped by limit).
	hits, err = svc.SearchCompaniesForMerge(ctx, org.ID, "", "", 20)
	if err != nil {
		t.Fatalf("SearchCompaniesForMerge empty: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 companies, got %d", len(hits))
	}

	// Limit is enforced.
	hits, err = svc.SearchCompaniesForMerge(ctx, org.ID, "", "", 1)
	if err != nil {
		t.Fatalf("SearchCompaniesForMerge limit: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 company with limit=1, got %d", len(hits))
	}

	// Out-of-range limit is clamped to the default cap.
	hits, err = svc.SearchCompaniesForMerge(ctx, org.ID, "", "", 9999)
	if err != nil {
		t.Fatalf("SearchCompaniesForMerge clamp: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 companies after clamp, got %d", len(hits))
	}
}
