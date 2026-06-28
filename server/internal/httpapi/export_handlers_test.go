package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestExportHandler(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	// 1. Export API (no IDs)
	token := fetchCSRFCookie(t, client, ts.URL+"/invoices")
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/export", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeaderName, token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/export: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	// 2. Export templates API
	resp, err = client.Get(ts.URL + "/api/v1/export/templates")
	if err != nil {
		t.Fatalf("GET /api/v1/export/templates: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestExportAction(t *testing.T) {
	ts, client, srv := newTestServer(t)
	setupAndLogin(t, ts, client)

	invID := createTestInvoice(t, srv)
	token := fetchCSRFCookie(t, client, ts.URL+"/invoices")

	resp, err := client.PostForm(ts.URL+"/export", url.Values{
		csrfFormField: {token},
		"ids":          {invID},
		"type":         {"all"},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200, body = %s", resp.StatusCode, body)
	}
}

func TestExportAPI_Success(t *testing.T) {
	ts, client, srv := newTestServer(t)
	setupAndLogin(t, ts, client)

	// Create an invoice to export
	invID := createTestInvoice(t, srv)

	token := fetchCSRFCookie(t, client, ts.URL+"/export")
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/export", strings.NewReader(fmt.Sprintf(`{"ids":["%s"]}`, invID)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestExportAPI_Errors(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	token := fetchCSRFCookie(t, client, ts.URL+"/export")

	// 1. Invalid JSON
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/export", strings.NewReader(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	// 2. Empty IDs
	req, _ = http.NewRequest("POST", ts.URL+"/api/v1/export", strings.NewReader(`{"ids":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", token)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 5); got != "hello" {
		t.Errorf("truncate(hello, 5) = %q", got)
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Errorf("truncate(hello world, 5) = %q", got)
	}
}
