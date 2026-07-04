package service

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExportInvoices(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()

	// Fetch org and user
	var orgID string
	_ = svc.Store().DB().QueryRow("SELECT id FROM organizations LIMIT 1").Scan(&orgID)
	user, _ := svc.Authenticate(ctx, "admin@test.com", "secret123")

	// Seed templates
	if err := svc.SeedExportTemplates(ctx); err != nil {
		t.Fatalf("SeedExportTemplates: %v", err)
	}

	// Create a ready invoice
	pngData, _ := os.ReadFile(filepath.Join("..", "testdata", "sample.png"))
	if pngData == nil {
		pngData = []byte("fake png")
	}
	inv, _ := svc.ImportInvoice(ctx, user.ID, orgID, "test.png", bytes.NewReader(pngData))
	_, _ = svc.Store().DB().Exec("UPDATE invoices SET status = 'ready_for_export' WHERE id = ?", inv.ID)

	// 1. Export JSON (Quick format)
	var buf bytes.Buffer
	params := ExportParams{
		IDs:    []string{inv.ID},
		Format: "json",
	}
	res, err := svc.ExportInvoices(ctx, params, &buf)
	if err != nil {
		t.Fatalf("ExportInvoices JSON: %v", err)
	}
	if res.ContentType != "application/json" {
		t.Errorf("ContentType = %s", res.ContentType)
	}
	if buf.Len() == 0 {
		t.Error("empty output")
	}

	// 2. Export with system template
	buf.Reset()
	params.Format = ""
	params.TemplateID = "system_generic"
	res, err = svc.ExportInvoices(ctx, params, &buf)
	if err != nil {
		t.Fatalf("ExportInvoices template: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("empty output from template")
	}

	// 3. Mark exported
	params.MarkExported = true
	_, _ = svc.ExportInvoices(ctx, params, &buf)
	
	// We need to be in the right context for GetInvoiceForOrg
	// Actually GetInvoiceForOrg uses organizationID(ctx)
	// Let's just query DB directly for simplicity in test
	var status string
	_ = svc.Store().DB().QueryRow("SELECT status FROM invoices WHERE id = ?", inv.ID).Scan(&status)
	if status != "exported" {
		t.Errorf("status = %s, want exported", status)
	}
}

// TestT5_DoubleExportBlocked verifies P1-2: an already-exported invoice cannot
// be exported again without an explicit AllowReExport opt-in, preventing
// duplicate accounting postings. With AllowReExport=true it is allowed.
func TestT5_DoubleExportBlocked(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()

	var orgID string
	_ = svc.Store().DB().QueryRow("SELECT id FROM organizations LIMIT 1").Scan(&orgID)
	user, _ := svc.Authenticate(ctx, "admin@test.com", "secret123")

	if err := svc.SeedExportTemplates(ctx); err != nil {
		t.Fatalf("SeedExportTemplates: %v", err)
	}

	pngData, _ := os.ReadFile(filepath.Join("..", "testdata", "sample.png"))
	if pngData == nil {
		pngData = []byte("fake png")
	}
	inv, _ := svc.ImportInvoice(ctx, user.ID, orgID, "test.png", bytes.NewReader(pngData))
	_, _ = svc.Store().DB().Exec("UPDATE invoices SET status = 'ready_for_export' WHERE id = ?", inv.ID)

	params := ExportParams{IDs: []string{inv.ID}, Format: "json", MarkExported: true}
	var buf bytes.Buffer
	res, err := svc.ExportInvoices(ctx, params, &buf)
	if err != nil {
		t.Fatalf("first export: %v", err)
	}
	if res == nil || res.BatchID == "" {
		t.Fatalf("first export: expected non-empty BatchID, got %+v", res)
	}
	var batchCount, itemCount int
	_ = svc.Store().DB().QueryRow("SELECT COUNT(*) FROM export_batches WHERE id = ?", res.BatchID).Scan(&batchCount)
	_ = svc.Store().DB().QueryRow("SELECT COUNT(*) FROM export_batch_items WHERE batch_id = ? AND invoice_id = ?", res.BatchID, inv.ID).Scan(&itemCount)
	if batchCount != 1 {
		t.Errorf("export_batches rows for %s = %d, want 1", res.BatchID, batchCount)
	}
	if itemCount != 1 {
		t.Errorf("export_batch_items link = %d, want 1", itemCount)
	}

	// 1. Second export without AllowReExport must be rejected.
	buf.Reset()
	if _, err := svc.ExportInvoices(ctx, params, &buf); err == nil {
		t.Error("second export without AllowReExport: expected error, got nil")
	}

	// 2. Non-exportable status (processed) must be rejected.
	_, _ = svc.Store().DB().Exec("UPDATE invoices SET status = 'processed' WHERE id = ?", inv.ID)
	buf.Reset()
	procParams := ExportParams{IDs: []string{inv.ID}, Format: "json"}
	if _, err := svc.ExportInvoices(ctx, procParams, &buf); err == nil {
		t.Error("export of processed invoice: expected error, got nil")
	}

	// 3. Re-export with AllowReExport on an exported invoice must succeed.
	_, _ = svc.Store().DB().Exec("UPDATE invoices SET status = 'exported' WHERE id = ?", inv.ID)
	buf.Reset()
	reParams := ExportParams{IDs: []string{inv.ID}, Format: "json", AllowReExport: true}
	if _, err := svc.ExportInvoices(ctx, reParams, &buf); err != nil {
		t.Errorf("re-export with AllowReExport: %v", err)
	}
}
