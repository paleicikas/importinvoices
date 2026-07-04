package service

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/reqctx"
)

// fakeWorker records Queue calls so tests can assert re-enqueue behavior.
type fakeWorker struct {
	mu  sync.Mutex
	ids []string
}

func (f *fakeWorker) Queue(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ids = append(f.ids, id)
}

func (f *fakeWorker) IDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.ids))
	copy(out, f.ids)
	return out
}

// TestT4_RecoverStuckInvoices verifies P1-1: after a restart, stale "processing"
// invoices are reset to "pending" and all "pending" invoices are re-enqueued to
// the worker. A fresh (in-flight) "processing" invoice must be left alone so an
// active worker is not double-scheduled.
func TestT4_RecoverStuckInvoices(t *testing.T) {
	svc, store, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()
	org, _ := svc.GetOrganization(ctx)
	user, _ := svc.Authenticate(ctx, "admin@test.com", "secret123")
	ctx = reqctx.WithOrganization(ctx, org)

	mk := func(id string) {
		seller := "Seller " + id
		inv := &domain.Invoice{
			ID:         id,
			UserID:     user.ID,
			OrgID:      org.ID,
			Status:     "pending",
			Filename:   id + ".pdf",
			Checksum:   "sum-" + id,
			SellerName: &seller,
		}
		if err := svc.CreateInvoice(ctx, inv); err != nil {
			t.Fatalf("CreateInvoice %s: %v", id, err)
		}
	}

	mk("inv-stale")   // processing, old updated_at -> must reset to pending + enqueue
	mk("inv-fresh")   // processing, recent updated_at -> must stay processing, no enqueue
	mk("inv-pending") // pending -> must be enqueued as-is

	now := time.Now().Unix()
	stale := now - int64(10*60) // 10 min ago, beyond 5 min threshold
	db := store.DB()

	for _, q := range []struct{ id, status string; ts int64 }{
		{"inv-stale", "processing", stale},
		{"inv-fresh", "processing", now},
		{"inv-pending", "pending", now},
	} {
		if _, err := db.Exec(`UPDATE invoices SET status = ?, updated_at = ? WHERE id = ?`, q.status, q.ts, q.id); err != nil {
			t.Fatalf("set status %s: %v", q.id, err)
		}
	}

	fw := &fakeWorker{}
	svc.SetWorker(fw)

	if err := svc.RecoverStuckInvoices(ctx); err != nil {
		t.Fatalf("RecoverStuckInvoices: %v", err)
	}

	// Status assertions.
	var status string
	for _, c := range []struct{ id, want string }{
		{"inv-stale", "pending"},
		{"inv-fresh", "processing"},
		{"inv-pending", "pending"},
	} {
		if err := db.QueryRow(`SELECT status FROM invoices WHERE id = ?`, c.id).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != c.want {
			t.Errorf("%s status = %q, want %q", c.id, status, c.want)
		}
	}

	// Enqueue assertions: stale + pending enqueued, fresh not.
	got := fw.IDs()
	sort.Strings(got)
	want := []string{"inv-pending", "inv-stale"}
	sort.Strings(want)
	if len(got) != len(want) || (len(got) > 0 && (got[0] != want[0] || got[len(got)-1] != want[len(want)-1])) {
		t.Errorf("enqueued = %v, want %v (fresh processing must NOT be enqueued)", got, want)
	}
}
