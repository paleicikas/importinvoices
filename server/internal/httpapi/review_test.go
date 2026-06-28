package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestReviewHandlers(t *testing.T) {
	ts, client, srv := newTestServer(t)
	setupAndLogin(t, ts, client)

	// 1. Review start (no invoices)
	resp, err := client.Get(ts.URL + "/invoices/review")
	if err != nil {
		t.Fatalf("GET /invoices/review: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "/invoices?tab=ready") {
		t.Errorf("Location = %s", loc)
	}

	// 2. Review page (with invoice)
	invID := createTestInvoice(t, srv)
	resp, err = client.Get(ts.URL + "/invoices/" + invID)
	if err != nil {
		t.Fatalf("GET /invoices/%s: %v", invID, err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// 3. Update invoice
	token := fetchCSRFCookie(t, client, ts.URL+"/invoices/"+invID)
	resp, err = client.PostForm(ts.URL+"/invoices/"+invID, url.Values{
		"invoice_number":           {"INV-123"},
		"items[0].description":    {"Item 1"},
		"items[0].quantity":       {"1"},
		"items[0].unit_price":     {"100"},
		"items[0].vat_amount":     {"21"},
		"items[0].vat_rate":       {"21"},
		"items[0].vat_classifier": {"PVM1"},
		csrfFormField:              {token},
	})
	if err != nil {
		t.Fatalf("POST /invoices/%s: %v", invID, err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}

	// Verify update
	_, items, err := srv.svc.GetInvoice(context.Background(), invID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].VatClassifier == nil || *items[0].VatClassifier != "PVM1" {
		t.Errorf("VatClassifier = %v, want PVM1", items[0].VatClassifier)
	}
	if items[0].VatAmount == nil || *items[0].VatAmount != 21 {
		t.Errorf("VatAmount = %v, want 21", items[0].VatAmount)
	}

	// 4. Confirm invoice
	resp, err = client.PostForm(ts.URL+"/invoices/"+invID+"/confirm", url.Values{
		csrfFormField: {token},
	})
	if err != nil {
		t.Fatalf("POST /invoices/%s/confirm: %v", invID, err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}

	// 5. Reprocess invoice
	resp, err = client.PostForm(ts.URL+"/invoices/"+invID+"/reprocess", url.Values{
		csrfFormField: {token},
	})
	if err != nil {
		t.Fatalf("POST /invoices/%s/reprocess: %v", invID, err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}

	// 6. Review page (not found)
	resp, err = client.Get(ts.URL + "/invoices/missing")
	if err != nil {
		t.Fatalf("GET /invoices/missing: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestReviewStart(t *testing.T) {
	ts, client, srv := newTestServer(t)
	setupAndLogin(t, ts, client)

	invID := createTestInvoice(t, srv)

	resp, err := client.Get(ts.URL + "/invoices/review")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	// Should redirect to the first invoice review page
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/invoices/"+invID) {
		t.Errorf("location = %s, want to contain /invoices/%s", loc, invID)
	}
}

func TestReviewHandlers_Errors(t *testing.T) {
	ts, client, srv := newTestServer(t)
	setupAndLogin(t, ts, client)

	invID := createTestInvoice(t, srv)
	token := fetchCSRFCookie(t, client, ts.URL+"/invoices/"+invID)

	// Test confirm non-existent
	resp := postForm(t, client, ts.URL+"/invoices/missing/confirm", url.Values{
		csrfFormField: {token},
	})
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}

	// Test reprocess non-existent
	resp = postForm(t, client, ts.URL+"/invoices/missing/reprocess", url.Values{
		csrfFormField: {token},
	})
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}

	// Test update non-existent
	resp = postForm(t, client, ts.URL+"/invoices/missing", url.Values{
		csrfFormField: {token},
	})
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
