package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/media"
	"github.com/paleicikas/importinvoices/server/internal/service"
	"github.com/paleicikas/importinvoices/server/internal/storage"
	"github.com/paleicikas/importinvoices/server/internal/webui"
)

func newTestServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()

	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	strg, err := storage.New(filepath.Join(dir, "files"))
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	mediaSvc := media.New(filepath.Join(dir, "temp"))
	svc := service.New(store, strg, mediaSvc)
	if err := svc.SetSetting(context.Background(), "openai_api_key", "sk-test"); err != nil {
		t.Fatalf("set setting: %v", err)
	}

	render, err := webui.NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	srv := NewServer(svc, render, strg.BasePath(), 10<<20, nil)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return ts, client
}

func csrfTokenFromJar(client *http.Client, baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	for _, c := range client.Jar.Cookies(u) {
		if c.Name == csrfCookieName {
			return c.Value
		}
	}
	return ""
}

func discardResponseBody(t *testing.T, resp *http.Response) {
	t.Helper()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("drain response body: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("close response body: %v", err)
	}
}

func fetchCSRFCookie(t *testing.T, client *http.Client, pageURL string) string {
	t.Helper()
	resp, err := client.Get(pageURL)
	if err != nil {
		t.Fatalf("GET %s: %v", pageURL, err)
	}
	discardResponseBody(t, resp)
	return csrfTokenFromJar(client, pageURL)
}

func postJSON(t *testing.T, client *http.Client, targetURL, csrfToken, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, targetURL, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeaderName, csrfToken)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", targetURL, err)
	}
	return resp
}

func setupAndLogin(t *testing.T, ts *httptest.Server, client *http.Client) {
	t.Helper()

	setupToken := fetchCSRFCookie(t, client, ts.URL+"/setup")
	setupBody := `{
		"org_title":"Test Org",
		"admin_name":"Admin",
		"admin_email":"admin@test.com",
		"admin_password":"secret123"
	}`
	resp := postJSON(t, client, ts.URL+"/api/v1/setup", setupToken, setupBody)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("setup status = %d", resp.StatusCode)
	}

	loginToken := fetchCSRFCookie(t, client, ts.URL+"/login")
	resp = postJSON(t, client, ts.URL+"/api/v1/login", loginToken, `{"email":"admin@test.com","password":"secret123"}`)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", resp.StatusCode)
	}
}

func TestUnauthenticatedInvoicesRedirectsBeforeSetup(t *testing.T) {
	ts, client := newTestServer(t)

	resp, err := client.Get(ts.URL + "/invoices")
	if err != nil {
		t.Fatalf("GET /invoices: %v", err)
	}
	discardResponseBody(t, resp)

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want redirect", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/setup" {
		t.Fatalf("Location = %q, want /setup", loc)
	}
}

func TestUnauthenticatedInvoicesRedirectsToLogin(t *testing.T) {
	ts, client := newTestServer(t)
	setupAndLogin(t, ts, client)

	jar, _ := cookiejar.New(nil)
	loggedOut := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := loggedOut.Get(ts.URL + "/invoices")
	if err != nil {
		t.Fatalf("GET /invoices: %v", err)
	}
	discardResponseBody(t, resp)

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want redirect to login", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("Location = %q, want /login", loc)
	}
}

func TestSetupLoginAndHome(t *testing.T) {
	ts, client := newTestServer(t)
	setupAndLogin(t, ts, client)

	resp, err := client.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("close body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %q", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "Home") && !strings.Contains(string(body), "Invoices") {
		t.Fatalf("home page missing expected content: %q", body[:min(200, len(body))])
	}
}

func TestSetupStatus(t *testing.T) {
	ts, client := newTestServer(t)

	resp, err := client.Get(ts.URL + "/api/v1/setup/status")
	if err != nil {
		t.Fatalf("GET setup status: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var status struct {
		NeedsSetup bool `json:"needs_setup"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !status.NeedsSetup {
		t.Fatal("expected needs_setup=true before setup")
	}

	setupAndLogin(t, ts, client)

	resp, err = client.Get(ts.URL + "/api/v1/setup/status")
	if err != nil {
		t.Fatalf("GET setup status after login: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.NeedsSetup {
		t.Fatal("expected needs_setup=false after setup")
	}
}

func TestUploadRejectsInvalidFileContent(t *testing.T) {
	ts, client := newTestServer(t)
	setupAndLogin(t, ts, client)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("files", "invoice.pdf")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("not a real pdf")); err != nil {
		t.Fatalf("write body: %v", err)
	}
	if err := writer.WriteField(csrfFormField, csrfTokenFromJar(client, ts.URL)); err != nil {
		t.Fatalf("write csrf field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/upload", &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	discardResponseBody(t, resp)

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want redirect after failed upload", resp.StatusCode)
	}

	var flashMsg string
	for _, c := range resp.Cookies() {
		if c.Name == "flash" {
			flashMsg = c.Value
			break
		}
	}
	if !strings.Contains(flashMsg, "unsupported file format") {
		t.Fatalf("flash = %q, want unsupported file format error", flashMsg)
	}
}

func TestLoginRateLimitBlocksAfterFailedAttempts(t *testing.T) {
	ts, client := newTestServer(t)
	setupAndLogin(t, ts, client)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client = &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	loginBody := `{"email":"admin@test.com","password":"wrong-password"}`
	for i := 0; i < loginRateLimitMax; i++ {
		token := fetchCSRFCookie(t, client, ts.URL+"/login")
		resp := postJSON(t, client, ts.URL+"/api/v1/login", token, loginBody)
		discardResponseBody(t, resp)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i+1, resp.StatusCode)
		}
	}

	token := fetchCSRFCookie(t, client, ts.URL+"/login")
	resp := postJSON(t, client, ts.URL+"/api/v1/login", token, loginBody)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 after rate limit exceeded", resp.StatusCode)
	}
	if retry := resp.Header.Get("Retry-After"); retry == "" {
		t.Fatal("expected Retry-After header on 429 response")
	}
}

func TestCSRFBlocksAuthenticatedPOSTWithoutToken(t *testing.T) {
	ts, client := newTestServer(t)
	setupAndLogin(t, ts, client)

	resp, err := client.PostForm(ts.URL+"/settings", url.Values{
		"llm_provider": {"openai"},
	})
	if err != nil {
		t.Fatalf("POST /settings: %v", err)
	}
	discardResponseBody(t, resp)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 without CSRF token", resp.StatusCode)
	}
}

func TestLoginRateLimitBlocksSpoofedForwardedFor(t *testing.T) {
	ts, client := newTestServer(t)
	setupAndLogin(t, ts, client)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client = &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	loginBody := `{"email":"admin@test.com","password":"wrong-password"}`
	for i := 0; i < loginRateLimitMax; i++ {
		token := fetchCSRFCookie(t, client, ts.URL+"/login")
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/login", strings.NewReader(loginBody))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(csrfHeaderName, token)
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i+1))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/login: %v", err)
		}
		discardResponseBody(t, resp)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i+1, resp.StatusCode)
		}
	}

	token := fetchCSRFCookie(t, client, ts.URL+"/login")
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/login", strings.NewReader(loginBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeaderName, token)
	req.Header.Set("X-Forwarded-For", "203.0.113.99")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/login: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 when spoofing X-Forwarded-For after limit", resp.StatusCode)
	}
}

func TestLoginSessionCookieSecureBehindHTTPSProxy(t *testing.T) {
	ts, client := newTestServer(t)
	setupAndLogin(t, ts, client)

	jar, _ := cookiejar.New(nil)
	client = &http.Client{Jar: jar}

	loginToken := fetchCSRFCookie(t, client, ts.URL+"/login")
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/login", strings.NewReader(`{"email":"admin@test.com","password":"secret123"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeaderName, loginToken)
	req.Header.Set("X-Forwarded-Proto", "https")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/login: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", resp.StatusCode)
	}

	for _, c := range resp.Cookies() {
		if c.Name == "session_token" {
			if !c.Secure {
				t.Fatal("expected Secure session cookie when X-Forwarded-Proto is https")
			}
			return
		}
	}
	t.Fatal("session_token cookie not set")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
