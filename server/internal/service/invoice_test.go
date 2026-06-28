package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/reqctx"
	"github.com/paleicikas/importinvoices/server/internal/storage"
)

func TestInvoice(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-invoice-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	dbPath := filepath.Join(tempDir, "test.db")
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	strg, _ := storage.New(filepath.Join(tempDir, "storage"))
	svc := New(store, strg, nil)

	ctx := context.Background()

	// Create user and org
	user, _ := svc.CreateUser(ctx, "test@example.com", "password1", "Test User")
	org, _ := svc.CreateOrganization(ctx, "Test Org")

	// Test CreateInvoice
	inv := &domain.Invoice{
		UserID:      user.ID,
		OrgID:       org.ID,
		Status:      "pending",
		Filename:    "test.pdf",
		Checksum:    "123456",
		StoragePath: "invoices/test.pdf",
	}
	err = svc.CreateInvoice(ctx, inv)
	if err != nil {
		t.Fatalf("failed to create invoice: %v", err)
	}

	// Test GetInvoice
	gotInv, items, err := svc.GetInvoice(ctx, inv.ID)
	if err != nil {
		t.Fatalf("failed to get invoice: %v", err)
	}
	if gotInv.Filename != "test.pdf" {
		t.Errorf("expected filename test.pdf, got %s", gotInv.Filename)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}

	// Test ListInvoices
	invoices, _, err := svc.ListInvoices(ctx, InvoiceListParams{Limit: 10})
	if err != nil {
		t.Fatalf("failed to list invoices: %v", err)
	}
	if len(invoices) != 1 {
		t.Errorf("expected 1 invoice, got %d", len(invoices))
	}
}

func TestInvoiceReviewQueue(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-review-queue-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	dbPath := filepath.Join(tempDir, "test.db")
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	strg, _ := storage.New(filepath.Join(tempDir, "storage"))
	svc := New(store, strg, nil)

	user, _ := svc.CreateUser(context.Background(), "review@example.com", "password1", "Review User")
	org, _ := svc.CreateOrganization(context.Background(), "Review Org")
	ctx := reqctx.WithOrganization(context.Background(), org)

	createProcessed := func(id string, createdAt int64) {
		_, err := store.DB().Exec(`
			INSERT INTO invoices (id, user_id, org_id, status, filename, checksum, storage_path, created_at, updated_at)
			VALUES (?, ?, ?, 'processed', ?, ?, ?, ?, ?)`,
			id, user.ID, org.ID, id+".pdf", id, "storage/"+id, createdAt, createdAt)
		if err != nil {
			t.Fatalf("failed to insert invoice %s: %v", id, err)
		}
	}

	createProcessed("inv-newest", 300)
	createProcessed("inv-middle", 200)
	createProcessed("inv-oldest", 100)

	middle, _, err := svc.GetInvoice(ctx, "inv-middle")
	if err != nil {
		t.Fatal(err)
	}

	nextID, err := svc.NextUnconfirmedInvoiceID(ctx, middle.CreatedAt, middle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if nextID != "inv-oldest" {
		t.Fatalf("expected next inv-oldest, got %q", nextID)
	}

	prevID, err := svc.PreviousUnconfirmedInvoiceID(ctx, middle.CreatedAt, middle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if prevID != "inv-newest" {
		t.Fatalf("expected prev inv-newest, got %q", prevID)
	}

	nextAfterConfirm, err := svc.NextUnconfirmedInvoiceID(ctx, middle.CreatedAt, middle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if nextAfterConfirm != "inv-oldest" {
		t.Fatalf("expected next after confirm position inv-oldest, got %q", nextAfterConfirm)
	}

	firstID, err := svc.GetFirstUnconfirmedInvoiceID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if firstID != "inv-newest" {
		t.Fatalf("expected first inv-newest, got %q", firstID)
	}
}

func TestInvoiceListFilters(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()
	user, _ := svc.Authenticate(ctx, "admin@test.com", "secret123")
	org, _ := svc.GetOrganization(ctx)
	ctx = reqctx.WithOrganization(ctx, org)

	// Create some invoices
	for i := 1; i <= 5; i++ {
		inv := &domain.Invoice{
			ID:          fmt.Sprintf("inv-%d", i),
			UserID:      user.ID,
			OrgID:       org.ID,
			Status:      "processed",
			Filename:    fmt.Sprintf("test-%d.pdf", i),
			Checksum:    fmt.Sprintf("sum-%d", i),
			SellerName:  strPtr(fmt.Sprintf("Seller %d", i)),
			AmountWithVat: floatPtr(float64(i * 100)),
		}
		_ = svc.CreateInvoice(ctx, inv)
		// Set status directly as CreateInvoice sets it from inv.Status but we might need to update it
		_, _ = svc.Store().DB().Exec("UPDATE invoices SET seller_name = ?, amount_with_vat = ?, status = 'processed' WHERE id = ?", *inv.SellerName, *inv.AmountWithVat, inv.ID)
	}

	// 1. Search
	list, total, err := svc.ListInvoices(ctx, InvoiceListParams{Search: "Seller 1"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("search total = %d, want 1", total)
	}

	// 2. Column Filter (Exact)
	params := InvoiceListParams{
		ColumnFilters: map[int][]string{
			6: {"Seller 2"}, // SellerName
		},
	}
	list, total, err = svc.ListInvoices(ctx, params)
	if total != 1 || list[0].ID != "inv-2" {
		t.Errorf("column filter total = %d, want 1", total)
	}

	// 3. Tab filter
	list, total, err = svc.ListInvoices(ctx, InvoiceListParams{Tab: "ready"})
	if total != 5 {
		t.Errorf("tab filter total = %d, want 5", total)
	}
}

func TestUpdateInvoice(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()
	org, _ := svc.GetOrganization(ctx)
	user, _ := svc.Authenticate(ctx, "admin@test.com", "secret123")
	ctx = reqctx.WithOrganization(ctx, org)

	inv := &domain.Invoice{ID: "inv-1", UserID: user.ID, OrgID: org.ID, Status: "processed"}
	_ = svc.CreateInvoice(ctx, inv)

	updated := *inv
	updated.SeriesAndNumber = strPtr("SN-123")
	items := []domain.InvoiceItem{
		{ID: "item-1", Description: strPtr("Item 1"), Quantity: floatPtr(1), UnitPrice: floatPtr(100)},
	}

	err := svc.UpdateInvoice(ctx, &updated, items)
	if err != nil {
		t.Fatalf("UpdateInvoice: %v", err)
	}

	gotInv, gotItems, _ := svc.GetInvoice(ctx, inv.ID)
	if gotInv.SeriesAndNumber == nil || *gotInv.SeriesAndNumber != "SN-123" {
		t.Errorf("SeriesAndNumber mismatch")
	}
	if len(gotItems) != 1 {
		t.Errorf("got %d items, want 1", len(gotItems))
	}
}

func TestConfirmAndReprocess(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()
	org, _ := svc.GetOrganization(ctx)
	user, _ := svc.Authenticate(ctx, "admin@test.com", "secret123")
	ctx = reqctx.WithOrganization(ctx, org)

	inv := &domain.Invoice{ID: "inv-1", UserID: user.ID, OrgID: org.ID, Status: "processed"}
	_ = svc.CreateInvoice(ctx, inv)
	_, _ = svc.Store().DB().Exec("UPDATE invoices SET status = 'processed' WHERE id = ?", inv.ID)

	// Confirm
	err := svc.ConfirmInvoice(ctx, inv.ID)
	if err != nil {
		t.Fatalf("ConfirmInvoice: %v", err)
	}
	got, err := svc.GetInvoiceForOrg(ctx, inv.ID)
	if err != nil {
		t.Fatalf("GetInvoiceForOrg: %v", err)
	}
	if got.Status != "ready_for_export" {
		t.Errorf("status = %s, want ready_for_export", got.Status)
	}

	// Reprocess
	err = svc.ScheduleReprocess(ctx, inv.ID)
	if err != nil {
		t.Fatalf("ScheduleReprocess: %v", err)
	}
	got, _ = svc.GetInvoiceForOrg(ctx, inv.ID)
	if got.Status != "pending" {
		t.Errorf("status = %s, want pending", got.Status)
	}
}
