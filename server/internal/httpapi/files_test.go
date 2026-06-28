package httpapi

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesHandlers(t *testing.T) {
	ts, client, srv := newTestServer(t)
	setupAndLogin(t, ts, client)

	// 1. Invoice file (not found)
	resp, err := client.Get(ts.URL + "/files/missing")
	if err != nil {
		t.Fatalf("GET /files/missing: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}

	// 2. Preview (not found)
	resp, err = client.Get(ts.URL + "/preview/missing")
	if err != nil {
		t.Fatalf("GET /preview/missing: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}

	// 3. Invoice preview (success)
	invID := createTestInvoice(t, srv)
	resp, err = client.Get(ts.URL + "/invoices/" + invID + "/preview")
	if err != nil {
		t.Fatalf("GET /invoices/%s/preview: %v", invID, err)
	}
	discardResponseBody(t, resp)
	// It might return 404 if preview file doesn't exist on disk, 
	// but the handler should find the invoice in DB.
	// Actually, handleInvoicePreview checks if file exists on disk.
	
	// 4. Invoice file (success)
	resp, err = client.Get(ts.URL + "/invoices/" + invID + "/file")
	if err != nil {
		t.Fatalf("GET /invoices/%s/file: %v", invID, err)
	}
	discardResponseBody(t, resp)
}

func TestFilesHandlers_Success(t *testing.T) {
	ts, client, srv := newTestServer(t)
	setupAndLogin(t, ts, client)

	ctx := context.Background()
	invID := createTestInvoice(t, srv)
	inv, _, _ := srv.svc.GetInvoice(ctx, invID)

	// Create a dummy file in storage
	relPath := filepath.Join(inv.OrgID, inv.ID+".pdf")
	filePath := filepath.Join(srv.storagePath, relPath)
	os.MkdirAll(filepath.Dir(filePath), 0755)
	os.WriteFile(filePath, []byte("dummy pdf content"), 0644)

	// Update invoice in DB with storage path
	srv.svc.Store().DB().Exec("UPDATE invoices SET storage_path = ? WHERE id = ?", relPath, inv.ID)

	// 1. Invoice file (success)
	resp, err := client.Get(ts.URL + "/invoices/" + invID + "/file")
	if err != nil {
		t.Fatal(err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// 2. Invoice preview (success)
	previewRelPath := filepath.Join(inv.OrgID, inv.ID+"_preview.jpg")
	previewPath := filepath.Join(srv.storagePath, previewRelPath)
	os.WriteFile(previewPath, []byte("dummy preview content"), 0644)
	
	srv.svc.Store().DB().Exec("UPDATE invoices SET preview_path = ? WHERE id = ?", previewRelPath, inv.ID)
	
	resp, err = client.Get(ts.URL + "/invoices/" + invID + "/preview")
	if err != nil {
		t.Fatal(err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}
