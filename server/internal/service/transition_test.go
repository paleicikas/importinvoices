package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/reqctx"
)

// TestT3_StatusTransitionGuards verifies P1-4: status transitions are guarded so
// that only eligible source statuses can be confirmed, reprocessed, or edited,
// and UpdateInvoice is scoped to the caller's organization (no cross-org update).
func TestT3_StatusTransitionGuards(t *testing.T) {
	svc, store, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()
	org, _ := svc.GetOrganization(ctx)
	user, _ := svc.Authenticate(ctx, "admin@test.com", "secret123")
	ctx = reqctx.WithOrganization(ctx, org)

	db := store.DB()

	mk := func(id, status string) {
		seller := "Seller " + id
		inv := &domain.Invoice{
			ID:         id,
			UserID:     user.ID,
			OrgID:      org.ID,
			Status:     "pending",
			Filename:   id + ".pdf",
			Checksum:   "sum-" + id,
			StoragePath: id + ".pdf",
			SellerName: &seller,
		}
		if err := svc.CreateInvoice(ctx, inv); err != nil {
			t.Fatalf("CreateInvoice %s: %v", id, err)
		}
		if _, err := db.Exec(`UPDATE invoices SET status = ? WHERE id = ?`, status, id); err != nil {
			t.Fatalf("set status %s: %v", id, err)
		}
	}

	// 1. ConfirmInvoice: only processed can be confirmed.
	for _, st := range []string{"pending", "exported", "duplicate", "failed"} {
		id := "confirm-" + st
		mk(id, st)
		err := svc.ConfirmInvoice(ctx, id)
		if err == nil {
			t.Errorf("ConfirmInvoice from %s: expected error, got nil", st)
		}
	}
	id := "confirm-processed"
	mk(id, "processed")
	if err := svc.ConfirmInvoice(ctx, id); err != nil {
		t.Errorf("ConfirmInvoice from processed: %v", err)
	}

	// 2. ScheduleReprocess: failed/processed/ready_for_export allowed; exported
	// only with allowUnExport; duplicate never.
	for _, st := range []string{"failed", "processed", "ready_for_export"} {
		id := "reproc-" + st
		mk(id, st)
		if err := svc.ScheduleReprocess(ctx, id, false); err != nil {
			t.Errorf("ScheduleReprocess from %s: %v", st, err)
		}
	}
	mk("reproc-exported", "exported")
	if err := svc.ScheduleReprocess(ctx, "reproc-exported", false); err == nil {
		t.Error("ScheduleReprocess exported without allowUnExport: expected error, got nil")
	}
	if _, err := db.Exec(`UPDATE invoices SET status = 'exported' WHERE id = 'reproc-exported'`); err != nil {
		t.Fatal(err)
	}
	if err := svc.ScheduleReprocess(ctx, "reproc-exported", true); err != nil {
		t.Errorf("ScheduleReprocess exported with allowUnExport: %v", err)
	}
	mk("reproc-dup", "duplicate")
	if err := svc.ScheduleReprocess(ctx, "reproc-dup", false); err == nil {
		t.Error("ScheduleReprocess duplicate: expected error, got nil")
	}

	// 3. UpdateInvoice is org-scoped: an invoice in a foreign org cannot be
	// updated from this org's context.
	otherOrgID := uuid.New().String()
	if _, err := db.Exec(`INSERT INTO organizations (id, title) VALUES (?, 'Other Org')`, otherOrgID); err != nil {
		t.Fatalf("insert other org: %v", err)
	}
	otherInv := &domain.Invoice{
		ID:          "inv-other-org",
		UserID:      user.ID,
		OrgID:       otherOrgID,
		Status:      "processed",
		Filename:    "other.pdf",
		Checksum:    "other-sum",
		StoragePath: "other.pdf",
	}
	if err := svc.CreateInvoice(ctx, otherInv); err != nil {
		t.Fatalf("CreateInvoice other org: %v", err)
	}
	// Update from orgA context (org.ID) targeting an invoice in otherOrgID -> rejected.
	updated := *otherInv
	sn := "X-999"
	updated.SeriesAndNumber = &sn
	if err := svc.UpdateInvoice(ctx, &updated, nil); err == nil {
		t.Error("UpdateInvoice cross-org: expected error, got nil")
	}
}
