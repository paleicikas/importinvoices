package worker

import (
	"context"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/media"
	"github.com/paleicikas/importinvoices/server/internal/processor"
	"github.com/paleicikas/importinvoices/server/internal/service"
	"github.com/paleicikas/importinvoices/server/internal/storage"
)

type mockProcessor struct {
	result *processor.Result
}

func (m *mockProcessor) Process(_ context.Context, imagePaths []string, _ []domain.VatClassifier) (*processor.Result, error) {
	if len(imagePaths) == 0 {
		return nil, os.ErrNotExist
	}
	return m.result, nil
}

func writeTestJPEG(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := jpeg.Encode(f, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
}

func TestProcessSavesSuccessfulExtraction(t *testing.T) {
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	filesDir := filepath.Join(dir, "files")
	strg, err := storage.New(filesDir)
	if err != nil {
		t.Fatal(err)
	}
	mediaSvc := media.New(filepath.Join(dir, "temp"))
	svc := service.New(store, strg, mediaSvc)
	svc.SetProcessorOverride(&mockProcessor{
		result: &processor.Result{
			IsInvoice:         true,
			Type:              "0",
			SeriesAndNumber:   "INV-100",
			Currency:          "EUR",
			IssueDate:         "2024-01-15",
			SellerCompanyName: "Seller UAB",
			BuyerCompanyName:  "Buyer UAB",
			Items: []processor.Item{
				{Name: "Service", Quantity: 1, AmountWithVat: 121},
			},
			AmountWithoutVat: 100,
			VatAmount:        21,
			AmountWithVat:    121,
			OcrText:          "Invoice OCR text",
		},
	})

	org, err := svc.CreateOrganization(context.Background(), "Org")
	if err != nil {
		t.Fatal(err)
	}
	user, err := svc.CreateUser(context.Background(), "u@test.com", "password1", "User")
	if err != nil {
		t.Fatal(err)
	}

	relPath := user.ID + "/invoice.jpg"
	absPath := filepath.Join(filesDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestJPEG(t, absPath)

	invoiceID := "inv-success-test"
	now := time.Now().Unix()
	_, err = store.DB().Exec(`
		INSERT INTO invoices (id, user_id, org_id, status, filename, checksum, storage_path, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', 'invoice.jpg', 'checksum-1', ?, ?, ?)`,
		invoiceID, user.ID, org.ID, relPath, now, now)
	if err != nil {
		t.Fatal(err)
	}

	w := New(store, svc, mediaSvc)
	if err := w.process(context.Background(), invoiceID); err != nil {
		t.Fatalf("process: %v", err)
	}

	var status, series, seller, ocr string
	err = store.DB().QueryRow(`
		SELECT status, series_and_number, seller_name, ocr_text
		FROM invoices WHERE id = ?`, invoiceID).Scan(&status, &series, &seller, &ocr)
	if err != nil {
		t.Fatal(err)
	}
	if status != "processed" {
		t.Fatalf("status = %q, want processed", status)
	}
	if series != "INV-100" {
		t.Fatalf("series_and_number = %q", series)
	}
	if seller != "Seller UAB" {
		t.Fatalf("seller_name = %q", seller)
	}
	if ocr != "Invoice OCR text" {
		t.Fatalf("ocr_text = %q", ocr)
	}

	var itemCount int
	if err := store.DB().QueryRow("SELECT COUNT(*) FROM invoice_items WHERE invoice_id = ?", invoiceID).Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	if itemCount != 1 {
		t.Fatalf("item count = %d, want 1", itemCount)
	}
}

func TestProcessMarksFailedWhenProcessorUnavailable(t *testing.T) {
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	strg, err := storage.New(filepath.Join(dir, "files"))
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(store, strg, nil)

	org, err := svc.CreateOrganization(context.Background(), "Org")
	if err != nil {
		t.Fatal(err)
	}
	user, err := svc.CreateUser(context.Background(), "u@test.com", "password1", "User")
	if err != nil {
		t.Fatal(err)
	}

	invoiceID := "inv-stuck-test"
	now := time.Now().Unix()
	_, err = store.DB().Exec(`
		INSERT INTO invoices (id, user_id, org_id, status, filename, checksum, storage_path, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', 'test.pdf', 'abc', 'missing.pdf', ?, ?)`,
		invoiceID, user.ID, org.ID, now, now)
	if err != nil {
		t.Fatal(err)
	}

	w := New(store, svc, nil)
	if err := w.process(context.Background(), invoiceID); err == nil {
		t.Fatal("expected processor error")
	}

	var status string
	if err := store.DB().QueryRow("SELECT status FROM invoices WHERE id = ?", invoiceID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("status = %q, want failed (not stuck on processing)", status)
	}
}
