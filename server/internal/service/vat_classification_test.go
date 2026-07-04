package service

import (
	"context"
	"strings"
	"testing"

	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/reqctx"
)

// insertTestClassifier inserts an org-scoped VAT classifier for testing.
func insertTestClassifier(t *testing.T, svc *Service, orgID, code string, tariff float64, reverseCharge bool) string {
	t.Helper()
	id := "vc-" + code
	active := 1
	rc := 0
	if reverseCharge {
		rc = 1
	}
	if _, err := svc.Store().DB().Exec(`INSERT INTO vat_classifiers (id, org_id, country, code, tariff, active, reverse_charge, include_in_isaf) VALUES (?, ?, 'LT', ?, ?, ?, ?, 0)`, id, orgID, code, tariff, active, rc); err != nil {
		t.Fatalf("insert classifier %s: %v", code, err)
	}
	return id
}

// TestT9_VatClassifierValidationAndTariff verifies P2-3.b/c/d: after a manual
// edit, the persisted item tariff comes from the matched classifier, an
// unknown classifier code sets vat_warning on the invoice, and a classifier
// still referenced by an invoice line cannot be deleted.
func TestT9_VatClassifierValidationAndTariff(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()
	org, _ := svc.GetOrganization(ctx)
	user, _ := svc.Authenticate(ctx, "admin@test.com", "secret123")
	ctx = reqctx.WithOrganization(ctx, org)

	// Org catalog: PVM1 (21%), PVM2 (9%), PVM5 (5%, reverse charge).
	pvm1 := insertTestClassifier(t, svc, org.ID, "PVM1", 21, false)
	pvm2 := insertTestClassifier(t, svc, org.ID, "PVM2", 9, false)
	insertTestClassifier(t, svc, org.ID, "PVM5", 5, true)

	inv := &domain.Invoice{
		ID:          "inv-vat-test",
		UserID:      user.ID,
		OrgID:       org.ID,
		Status:      "processed",
		Filename:    "vat.pdf",
		Checksum:    "vat-sum",
		StoragePath: "vat.pdf",
	}
	if err := svc.CreateInvoice(ctx, inv); err != nil {
		t.Fatalf("CreateInvoice: %v", err)
	}

	pvm1Code, pvmxCode := "PVM1", "PVMX"
	items := []domain.InvoiceItem{
		{ID: "it-1", InvoiceID: inv.ID, VatClassifier: &pvm1Code},
		{ID: "it-2", InvoiceID: inv.ID, VatClassifier: &pvmxCode}, // unknown
	}
	if err := svc.UpdateInvoice(ctx, inv, items); err != nil {
		t.Fatalf("UpdateInvoice: %v", err)
	}

	got, gotItems, err := svc.GetInvoice(ctx, inv.ID)
	if err != nil {
		t.Fatalf("GetInvoice: %v", err)
	}

	// vat_warning must mention the unknown code.
	if got.VatWarning == nil || !strings.Contains(*got.VatWarning, "PVMX") {
		t.Errorf("vat_warning = %v, want mention of PVMX", got.VatWarning)
	}

	// Item 1 tariff must be persisted as 21 (from PVM1); item 2 tariff nil.
	var found1, found2 bool
	for _, it := range gotItems {
		switch it.ID {
		case "it-1":
			found1 = true
			if it.Tariff == nil || *it.Tariff != 21 {
				t.Errorf("it-1 tariff = %v, want 21", it.Tariff)
			}
		case "it-2":
			found2 = true
			if it.Tariff != nil {
				t.Errorf("it-2 tariff = %v, want nil (unknown classifier)", it.Tariff)
			}
		}
	}
	if !found1 || !found2 {
		t.Fatalf("missing items: found1=%v found2=%v items=%v", found1, found2, gotItems)
	}

	// DeleteVatClassifier must refuse PVM1 (used by it-1).
	if err := svc.DeleteVatClassifier(ctx, pvm1, org.ID); err == nil {
		t.Error("DeleteVatClassifier PVM1: expected error (in use), got nil")
	}
	// PVM2 is unused and can be deleted.
	if err := svc.DeleteVatClassifier(ctx, pvm2, org.ID); err != nil {
		t.Errorf("DeleteVatClassifier PVM2 (unused): %v", err)
	}

	// After re-saving with only valid classifiers, vat_warning must clear.
	items = []domain.InvoiceItem{
		{ID: "it-1", InvoiceID: inv.ID, VatClassifier: &pvm1Code},
	}
	if err := svc.UpdateInvoice(ctx, inv, items); err != nil {
		t.Fatalf("UpdateInvoice re-save: %v", err)
	}
	got, _, _ = svc.GetInvoice(ctx, inv.ID)
	if got.VatWarning != nil && *got.VatWarning != "" {
		t.Errorf("vat_warning after fix = %q, want empty", *got.VatWarning)
	}
}
