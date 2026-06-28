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
