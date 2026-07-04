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
	"github.com/paleicikas/importinvoices/server/internal/export"
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

// TestT2_TotalPriceGrossAfterWorker verifies P0-1 fix: after worker processes an
// invoice (no manual edit), DB total_price must hold the GROSS amount (with VAT),
// and the export payload AmountWithVat must equal net + VAT. Before the fix,
// worker wrote net into total_price, causing silent export corruption.
func TestT2_TotalPriceGrossAfterWorker(t *testing.T) {
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
			SeriesAndNumber:   "INV-T2",
			Currency:          "EUR",
			IssueDate:         "2024-01-15",
			SellerCompanyName: "Seller UAB",
			BuyerCompanyName:  "Buyer UAB",
			Items: []processor.Item{
				{Name: "Service", Quantity: 1, AmountWithoutVat: 100, VatAmount: 21, AmountWithVat: 121},
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

	invoiceID := "inv-t2-test"
	now := time.Now().Unix()
	_, err = store.DB().Exec(`
		INSERT INTO invoices (id, user_id, org_id, status, filename, checksum, storage_path, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', 'invoice.jpg', 'checksum-t2', ?, ?, ?)`,
		invoiceID, user.ID, org.ID, relPath, now, now)
	if err != nil {
		t.Fatal(err)
	}

	w := New(store, svc, mediaSvc)
	if err := w.process(context.Background(), invoiceID); err != nil {
		t.Fatalf("process: %v", err)
	}

	// 1. DB total_price must be GROSS in cents (12100 = net 10000 + VAT 2100), not net.
	var totalPrice int64
	if err := store.DB().QueryRow(
		`SELECT total_price FROM invoice_items WHERE invoice_id = ?`, invoiceID,
	).Scan(&totalPrice); err != nil {
		t.Fatal(err)
	}
	if totalPrice != 12100 {
		t.Fatalf("DB total_price = %v cents, want 12100 (gross = net 10000 + VAT 2100)", totalPrice)
	}

	// 2. Export payload (no manual edit) must carry AmountWithVat = net + VAT.
	inv, items, err := svc.GetInvoice(context.Background(), invoiceID)
	if err != nil {
		t.Fatalf("GetInvoice: %v", err)
	}
	itemsMap := map[string][]domain.InvoiceItem{invoiceID: items}
	payload := export.BuildPayload([]domain.Invoice{*inv}, itemsMap, nil, export.InvoiceTypePurchases, "http://localhost:8080")
	if len(payload.PurchasesInvoices) != 1 || len(payload.PurchasesInvoices[0].Items) != 1 {
		t.Fatalf("expected 1 purchase invoice with 1 item, got %+v", payload.PurchasesInvoices)
	}
	got := payload.PurchasesInvoices[0].Items[0]
	if got.AmountWithVat != 121 {
		t.Errorf("AmountWithVat = %v, want 121", got.AmountWithVat)
	}
	if got.AmountWithoutVat != 100 {
		t.Errorf("AmountWithoutVat = %v, want 100", got.AmountWithoutVat)
	}
	if got.VatAmount != 21 {
		t.Errorf("VatAmount = %v, want 21", got.VatAmount)
	}
}

// TestT6_BusinessKeyDedup verifies P1-3: two invoices with different file bytes
// but the same (org, seller_vat, series_and_number) are detected as duplicates
// after AI processing. The first processes normally; the second is marked
// "duplicate" and linked to the first, even though the checksums differ.
func TestT6_BusinessKeyDedup(t *testing.T) {
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
			IsInvoice:                  true,
			Type:                       "0",
			SeriesAndNumber:            "INV-200",
			SellerCompanyName:          "Seller UAB",
			SellerVatIdentificationNumber: "LT123",
			Currency:                   "EUR",
			AmountWithVat:              121,
			Items: []processor.Item{
				{Name: "Service", Quantity: 1, AmountWithVat: 121},
			},
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

	now := time.Now().Unix()
	mkInv := func(id, checksum, relPath string) {
		absPath := filepath.Join(filesDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			t.Fatal(err)
		}
		writeTestJPEG(t, absPath)
		if _, err := store.DB().Exec(`
			INSERT INTO invoices (id, user_id, org_id, status, filename, checksum, storage_path, created_at, updated_at)
			VALUES (?, ?, ?, 'pending', ?, ?, ?, ?, ?)`,
			id, user.ID, org.ID, id+".jpg", checksum, relPath, now, now); err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}

	first := "inv-bk-1"
	second := "inv-bk-2"
	mkInv(first, "checksum-bk-1", user.ID+"/invoice1.jpg")
	mkInv(second, "checksum-bk-2", user.ID+"/invoice2.jpg")

	w := New(store, svc, mediaSvc)
	if err := w.process(context.Background(), first); err != nil {
		t.Fatalf("process first: %v", err)
	}
	if err := w.process(context.Background(), second); err != nil {
		t.Fatalf("process second: %v", err)
	}

	var firstStatus string
	_ = store.DB().QueryRow("SELECT status FROM invoices WHERE id = ?", first).Scan(&firstStatus)
	if firstStatus != "processed" {
		t.Errorf("first status = %s, want processed", firstStatus)
	}

	var secondStatus, dupOf string
	_ = store.DB().QueryRow("SELECT status, duplicate_of_id FROM invoices WHERE id = ?", second).Scan(&secondStatus, &dupOf)
	if secondStatus != "duplicate" {
		t.Errorf("second status = %s, want duplicate (business-key dup)", secondStatus)
	}
	if dupOf != first {
		t.Errorf("second duplicate_of_id = %s, want %s", dupOf, first)
	}
}
