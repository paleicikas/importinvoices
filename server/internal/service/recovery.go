package service

import (
	"context"
	"log"
	"time"
)

// staleProcessingThreshold is how long a "processing" invoice may be in flight
// before it is considered stuck (e.g. the process was killed mid-processing).
const staleProcessingThreshold = 5 * time.Minute

// RecoverStuckInvoices runs at server boot to recover from an unclean shutdown:
// invoices that were "processing" when the process died stay "processing"
// forever (the deferred status reset never ran), which blocks HTTP edit guards
// and prevents retries. It also re-enqueues any "pending" invoices because the
// worker queue is in-memory and is lost on restart.
//
// Steps:
//  1. Reset stale "processing" (updated_at older than staleProcessingThreshold)
//     back to "pending".
//  2. Enqueue every "pending" invoice to the worker so it gets reprocessed.
//
// Safe to call multiple times: recovery is idempotent on status, and enqueue
// just schedules work. Requires SetWorker to have been called; if no worker is
// configured this is a no-op for the enqueue step.
func (s *Service) RecoverStuckInvoices(ctx context.Context) error {
	cutoff := time.Now().Add(-staleProcessingThreshold).Unix()

	res, err := s.store.DB().ExecContext(ctx, `
		UPDATE invoices
		SET status = 'pending', updated_at = unixepoch()
		WHERE status = 'processing' AND updated_at < ?`, cutoff)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		log.Printf("RecoverStuckInvoices: reset %d stale processing invoice(s) to pending", n)
	}

	rows, err := s.store.DB().QueryContext(ctx, `
		SELECT id FROM invoices WHERE status = 'pending' ORDER BY created_at ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	enqueued := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		if s.worker != nil {
			s.worker.Queue(id)
			enqueued++
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if enqueued > 0 {
		log.Printf("RecoverStuckInvoices: enqueued %d pending invoice(s)", enqueued)
	}
	return nil
}
