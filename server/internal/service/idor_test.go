package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/reqctx"
)

// TestT17_IDORCrossOrgReads verifies P2-1: the ForOrg read methods reject
// cross-organization access. A record created in org B must not be readable
// from org A's context (returns sql.ErrNoRows / "export template not found").
func TestT17_IDORCrossOrgReads(t *testing.T) {
	svc, store, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()
	orgA, _ := svc.GetOrganization(ctx)
	user, _ := svc.Authenticate(ctx, "admin@test.com", "secret123")
	ctxA := reqctx.WithOrganization(ctx, orgA)

	db := store.DB()

	// Create a second organization and a user-less context for it.
	orgBID := uuid.New().String()
	if _, err := db.Exec(`INSERT INTO organizations (id, title) VALUES (?, 'Org B')`, orgBID); err != nil {
		t.Fatalf("insert orgB: %v", err)
	}

	// Company in org B.
	companyBID := "cmp-orgB"
	if _, err := db.Exec(`INSERT INTO companies (id, org_id, title, individual) VALUES (?, ?, 'B Co', 0)`, companyBID, orgBID); err != nil {
		t.Fatalf("insert companyB: %v", err)
	}

	// Org-scoped (non-system) export template in org B.
	tmplBID := "tmpl-orgB"
	if _, err := db.Exec(`INSERT INTO export_templates (id, org_id, type, title, active, is_system, is_favorite) VALUES (?, ?, 'file', 'B Template', 1, 0, 0)`, tmplBID, orgBID); err != nil {
		t.Fatalf("insert tmplB: %v", err)
	}

	// VAT classifier in org B.
	vcBID := "vc-orgB"
	if _, err := db.Exec(`INSERT INTO vat_classifiers (id, org_id, country, code, tariff, active, reverse_charge, include_in_isaf) VALUES (?, ?, 'LT', 'PVM9', 0, 1, 0, 0)`, vcBID, orgBID); err != nil {
		t.Fatalf("insert vcB: %v", err)
	}

	// Invoice in org B.
	invB := &domain.Invoice{
		ID:          "inv-orgB",
		UserID:      user.ID,
		OrgID:       orgBID,
		Status:      "processed",
		Filename:    "b.pdf",
		Checksum:    "b-sum",
		StoragePath: "b.pdf",
	}
	if err := svc.CreateInvoice(ctxA, invB); err != nil {
		t.Fatalf("CreateInvoice orgB: %v", err)
	}

	// 1. GetCompanyForOrg: org A must not see org B's company.
	if _, err := svc.GetCompanyForOrg(ctxA, companyBID); err == nil {
		t.Error("GetCompanyForOrg cross-org: expected error, got nil")
	}

	// 2. GetExportTemplateForOrg: org A must not see org B's non-system template.
	if _, _, err := svc.GetExportTemplateForOrg(ctxA, tmplBID); err == nil {
		t.Error("GetExportTemplateForOrg cross-org non-system: expected error, got nil")
	}

	// 3. GetVatClassifierForOrg: org A must not see org B's classifier.
	if _, err := svc.GetVatClassifierForOrg(ctxA, vcBID); err == nil {
		t.Error("GetVatClassifierForOrg cross-org: expected error, got nil")
	}

	// 4. GetInvoiceForOrg: org A must not see org B's invoice.
	if _, err := svc.GetInvoiceForOrg(ctxA, invB.ID); err == nil {
		t.Error("GetInvoiceForOrg cross-org: expected error, got nil")
	}

	// 5. Sanity: same records ARE visible from org B's context.
	ctxB := reqctx.WithOrganization(ctx, &domain.Organization{ID: orgBID, Title: "Org B"})
	if _, err := svc.GetCompanyForOrg(ctxB, companyBID); err != nil {
		t.Errorf("GetCompanyForOrg same-org: %v", err)
	}
	if _, _, err := svc.GetExportTemplateForOrg(ctxB, tmplBID); err != nil {
		t.Errorf("GetExportTemplateForOrg same-org: %v", err)
	}
	if _, err := svc.GetVatClassifierForOrg(ctxB, vcBID); err != nil {
		t.Errorf("GetVatClassifierForOrg same-org: %v", err)
	}
	if _, err := svc.GetInvoiceForOrg(ctxB, invB.ID); err != nil {
		t.Errorf("GetInvoiceForOrg same-org: %v", err)
	}
}
