package service

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// TestT1_ExportReconciliationWarning verifies P2-5.a: when an invoice's header
// totals disagree with the sum of its line items by more than EUR 0.01, the
// export result carries a reconciliation warning (and the export still
// succeeds).
func TestT1_ExportReconciliationWarning(t *testing.T) {
	svc, store, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()

	var orgID string
	_ = store.DB().QueryRow("SELECT id FROM organizations LIMIT 1").Scan(&orgID)
	user, _ := svc.Authenticate(ctx, "admin@test.com", "secret123")

	// Header says 100/21/121 but the only line sums to 50/10/60 -> mismatch.
	invID := "inv-recon"
	now := time.Now().Unix()
	series := "RECON-001"
	if _, err := store.DB().Exec(`
		INSERT INTO invoices (id, user_id, org_id, status, filename, checksum, storage_path, series_and_number,
			amount_without_vat, vat_amount, amount_with_vat, created_at, updated_at)
		VALUES (?, ?, ?, 'ready_for_export', 'r.pdf', 'rsum', 'r.pdf', ?, 10000, 2100, 12100, ?, ?)`,
		invID, user.ID, orgID, series, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`
		INSERT INTO invoice_items (id, invoice_id, total_price, vat_amount, created_at)
		VALUES ('it-r', ?, 6000, 1000, ?)`, invID, now); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	res, err := svc.ExportInvoices(ctx, ExportParams{IDs: []string{invID}, Format: "json"}, &buf)
	if err != nil {
		t.Fatalf("ExportInvoices: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Fatalf("expected reconciliation warning, got none")
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, series) {
			found = true
		}
	}
	if !found {
		t.Errorf("warning does not mention series %s: %v", series, res.Warnings)
	}

	// A matching invoice (header == line sums) must produce no warning.
	invOK := "inv-recon-ok"
	if _, err := store.DB().Exec(`
		INSERT INTO invoices (id, user_id, org_id, status, filename, checksum, storage_path, series_and_number,
			amount_without_vat, vat_amount, amount_with_vat, created_at, updated_at)
		VALUES (?, ?, ?, 'ready_for_export', 'ok.pdf', 'oksum', 'ok.pdf', 'OK-001', 5000, 1000, 6000, ?, ?)`,
		invOK, user.ID, orgID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`
		INSERT INTO invoice_items (id, invoice_id, total_price, vat_amount, created_at)
		VALUES ('it-ok', ?, 6000, 1000, ?)`, invOK, now); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	res2, err := svc.ExportInvoices(ctx, ExportParams{IDs: []string{invOK}, Format: "json"}, &buf)
	if err != nil {
		t.Fatalf("ExportInvoices ok: %v", err)
	}
	if len(res2.Warnings) != 0 {
		t.Errorf("matching invoice produced warnings: %v", res2.Warnings)
	}
}

// TestEscapeLike verifies P2-6.e: LIKE wildcards and the escape char are
// escaped so user search terms are matched literally (paired with ESCAPE '\').
func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"plain":      "plain",
		"50%":        "50\\%",
		"a_b":        "a\\_b",
		"C:\\dir":    "C:\\\\dir",
		"10%_off\\x": "10\\%\\_off\\\\x",
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestT20_LoadExportCompaniesError verifies P2-5.b: loadExportCompanies returns
// an error (not nil) when the organization cannot be resolved, and
// loadExportData propagates it instead of silently proceeding with no
// companies.
func TestT20_LoadExportCompaniesError(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	// No SetupUser: the context has no organization, so GetOrganization fails.
	ctx := context.Background()
	if _, err := svc.loadExportCompanies(ctx); err == nil {
		t.Error("loadExportCompanies with no org: expected error, got nil")
	}
	// loadExportData must propagate that error rather than returning nil.
	if _, _, _, err := svc.loadExportData(ctx, []string{"any"}, false); err == nil {
		t.Error("loadExportData with no org: expected error, got nil")
	}
}
