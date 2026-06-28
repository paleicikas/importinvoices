package service

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestImportInvoice(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()
	user, _ := svc.Authenticate(ctx, "admin@test.com", "secret123")
	orgID := "org-123" // SetupUser creates an org, but we don't return its ID easily. 
	// Let's check what org ID it created or just use one.
	// Actually, SetupUser in testutil.go doesn't return orgID.
	// I'll update testutil.go to return orgID or I'll fetch it.
	
	var orgTitle string
	_ = svc.Store().DB().QueryRow("SELECT id, title FROM organizations LIMIT 1").Scan(&orgID, &orgTitle)

	// 1. Successful import (PNG)
	pngData, err := os.ReadFile(filepath.Join("..", "testdata", "sample.png"))
	if err != nil {
		// Try alternative path for local running
		pngData, err = os.ReadFile(filepath.Join("server", "internal", "testdata", "sample.png"))
		if err != nil {
			t.Fatalf("read sample png: %v", err)
		}
	}
	
	inv, err := svc.ImportInvoice(context.Background(), user.ID, orgID, "invoice.png", bytes.NewReader(pngData))
	if err != nil {
		t.Fatalf("ImportInvoice PNG: %v", err)
	}
	if inv.Status != "pending" {
		t.Errorf("status = %s, want pending", inv.Status)
	}

	// 2. Duplicate import
	inv2, err := svc.ImportInvoice(context.Background(), user.ID, orgID, "invoice2.png", bytes.NewReader(pngData))
	if err != nil {
		t.Fatalf("ImportInvoice duplicate: %v", err)
	}
	if inv2.Status != "duplicate" {
		t.Errorf("status = %s, want duplicate", inv2.Status)
	}
	if inv2.DuplicateOfID == nil || *inv2.DuplicateOfID != inv.ID {
		t.Errorf("DuplicateOfID mismatch")
	}

	// 3. Extension mismatch
	_, err = svc.ImportInvoice(context.Background(), user.ID, orgID, "invoice.pdf", bytes.NewReader(pngData))
	if err == nil {
		t.Fatal("expected error for extension mismatch")
	}

	// 4. Invalid content
	_, err = svc.ImportInvoice(context.Background(), user.ID, orgID, "invoice.png", bytes.NewReader([]byte("not a png")))
	if err == nil {
		t.Fatal("expected error for invalid content")
	}
}
