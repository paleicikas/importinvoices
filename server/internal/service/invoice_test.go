package service

import (
	"context"
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
