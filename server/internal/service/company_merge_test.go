package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/reqctx"
)

func TestNormalizeVATCode(t *testing.T) {
	cases := map[string]string{
		"LT123456789":  "123456789",
		"lt123456789":  "123456789",
		"DE123":        "123",
		"123456789":    "123456789",
		"LT":           "LT", // too short to have a number after prefix
		"":             "",
		"  LT999  ":    "999",
	}
	for in, want := range cases {
		if got := normalizeVATCode(in); got != want {
			t.Errorf("normalizeVATCode(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestT_MergeCompanies verifies P2-4.f: merging moves invoice FK links from
// the source company to the target and deletes the source.
func TestT_MergeCompanies(t *testing.T) {
	svc, store, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()
	org, _ := svc.GetOrganization(ctx)
	user, _ := svc.Authenticate(ctx, "admin@test.com", "secret123")
	ctx = reqctx.WithOrganization(ctx, org)

	// Two companies with VAT codes.
	srcVAT, tgtVAT := "111111111", "222222222"
	srcID, err := svc.UpsertCompany(ctx, domain.Company{OrgID: org.ID, Title: "Source Co", VATCode: &srcVAT}, nil)
	if err != nil {
		t.Fatalf("upsert source: %v", err)
	}
	tgtID, err := svc.UpsertCompany(ctx, domain.Company{OrgID: org.ID, Title: "Target Co", VATCode: &tgtVAT}, nil)
	if err != nil {
		t.Fatalf("upsert target: %v", err)
	}

	// An invoice linked to the source as seller and another as buyer.
	now := time.Now().Unix()
	mkInv := func(id, sellerID, buyerID string) {
		s, b := (*string)(nil), (*string)(nil)
		if sellerID != "" {
			s = &sellerID
		}
		if buyerID != "" {
			b = &buyerID
		}
		if _, err := store.DB().Exec(`INSERT INTO invoices (id, user_id, org_id, status, filename, checksum, storage_path, seller_company_id, buyer_company_id, created_at, updated_at) VALUES (?, ?, ?, 'processed', ?, ?, ?, ?, ?, ?, ?)`,
			id, user.ID, org.ID, id+".pdf", id+"sum", id+".pdf", s, b, now, now); err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}
	mkInv("inv-seller", srcID, tgtID)
	mkInv("inv-buyer", tgtID, srcID)

	if err := svc.MergeCompanies(ctx, org.ID, srcID, tgtID); err != nil {
		t.Fatalf("MergeCompanies: %v", err)
	}

	// Source company is gone.
	var n int
	if err := store.DB().QueryRow("SELECT COUNT(*) FROM companies WHERE id = ?", srcID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("source company still exists after merge")
	}

	// Both invoices now point at the target for both seller and buyer.
	for _, invID := range []string{"inv-seller", "inv-buyer"} {
		var sellerID, buyerID sql.NullString
		if err := store.DB().QueryRow("SELECT seller_company_id, buyer_company_id FROM invoices WHERE id = ?", invID).Scan(&sellerID, &buyerID); err != nil {
			t.Fatal(err)
		}
		if !sellerID.Valid || sellerID.String != tgtID {
			t.Errorf("%s seller_company_id = %v, want %s", invID, sellerID.String, tgtID)
		}
		if !buyerID.Valid || buyerID.String != tgtID {
			t.Errorf("%s buyer_company_id = %v, want %s", invID, buyerID.String, tgtID)
		}
	}

	// Merging a company with itself must error.
	if err := svc.MergeCompanies(ctx, org.ID, tgtID, tgtID); err == nil {
		t.Error("MergeCompanies into itself: expected error, got nil")
	}
	// Merging a non-existent source must error.
	if err := svc.MergeCompanies(ctx, org.ID, uuid.New().String(), tgtID); err == nil {
		t.Error("MergeCompanies non-existent source: expected error, got nil")
	}
}
