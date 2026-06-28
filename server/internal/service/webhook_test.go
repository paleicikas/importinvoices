package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/paleicikas/importinvoices/server/internal/domain"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestWebhook(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()
	user, err := svc.Authenticate(ctx, "admin@test.com", "secret123")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	// 1. Mock webhook receiver
	var received bool
	svc.Webhook.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			received = true
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader([]byte("ok"))),
			}, nil
		}),
	}

	// 2. Set webhook URL
	urls := map[string]string{"invoice.exported": "http://example.com/hook"}
	urlsJSON, _ := json.Marshal(urls)
	urlsStr := string(urlsJSON)
	user.WebhookUrls = &urlsStr

	// Update user in DB directly
	_, err = svc.Store().DB().Exec("UPDATE users SET webhook_urls = ? WHERE id = ?", urlsStr, user.ID)
	if err != nil {
		t.Fatalf("update user: %v", err)
	}

	// 3. Send event
	inv := &domain.Invoice{ID: "inv-1"}
	_, _ = svc.Store().DB().Exec("INSERT INTO invoices (id, user_id, org_id, status, filename, checksum, storage_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		inv.ID, user.ID, "org-123", "ready_for_export", "test.pdf", "sum", "path", 0, 0)

	err = svc.Webhook.SendInvoiceEvent(ctx, user.ID, "invoice.exported", inv, "http://localhost")
	if err != nil {
		t.Fatalf("SendInvoiceEvent: %v", err)
	}
	if !received {
		t.Error("webhook not received")
	}

	// 4. Test missing event
	received = false
	err = svc.Webhook.SendInvoiceEvent(ctx, user.ID, "missing.event", inv, "")
	if err != nil {
		t.Fatalf("SendInvoiceEvent missing: %v", err)
	}
	if received {
		t.Error("webhook should not be received for missing event")
	}
}
