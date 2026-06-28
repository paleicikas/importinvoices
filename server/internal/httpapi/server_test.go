package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
)

func TestUnauthenticatedInvoicesRedirectsBeforeSetup(t *testing.T) {
	ts, client, _ := newTestServer(t)

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
	ts, client, _ := newTestServer(t)
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
	ts, client, _ := newTestServer(t)
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
	ts, client, _ := newTestServer(t)

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
	ts, client, _ := newTestServer(t)
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
	ts, client, _ := newTestServer(t)
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
	ts, client, _ := newTestServer(t)
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
	ts, client, _ := newTestServer(t)
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
	ts, client, _ := newTestServer(t)
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

func TestLogout(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	resp, err := client.Get(ts.URL + "/logout")
	if err != nil {
		t.Fatalf("GET /logout: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}

	// Verify redirect to login
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}

	// Verify session cookie cleared
	for _, c := range resp.Cookies() {
		if c.Name == "session_token" {
			if c.Value != "" || c.MaxAge != -1 {
				t.Errorf("session cookie not cleared: %+v", c)
			}
		}
	}
}

func TestInvoicesFilters(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	// Test various filters and sorting
	urls := []string{
		"/invoices?tab=processed",
		"/invoices?q=test",
		"/invoices?sort=1&dir=asc",
		"/invoices?f.1=Company",
	}

	for _, u := range urls {
		resp, err := client.Get(ts.URL + u)
		if err != nil {
			t.Fatalf("GET %s: %v", u, err)
		}
		discardResponseBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", u, resp.StatusCode)
		}
	}
}

func TestInvalidSessionRedirectsToLogin(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	// Manually set an invalid session cookie
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(ts.URL)
	jar.SetCookies(u, []*http.Cookie{
		{Name: "session_token", Value: "invalid"},
	})
	client.Jar = jar

	resp, err := client.Get(ts.URL + "/invoices")
	if err != nil {
		t.Fatalf("GET /invoices: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
}

func TestSetupAlreadyDone(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	token := fetchCSRFCookie(t, client, ts.URL+"/setup")
	resp := postJSON(t, client, ts.URL+"/api/v1/setup", token, `{"org_title":"Another"}`)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestIndexRedirect(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	resp, err := client.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestSetup_Success(t *testing.T) {
	ts, client, _ := newTestServer(t)
	// No setup yet

	token := fetchCSRFCookie(t, client, ts.URL+"/setup")
	resp := postJSON(t, client, ts.URL+"/api/v1/setup", token, `{
		"org_title": "Test Org",
		"admin_name": "Admin",
		"admin_email": "admin@test.com",
		"admin_password": "password123"
	}`)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// 2. Setup status (already done)
	resp, err := client.Get(ts.URL + "/api/v1/setup/status")
	if err != nil {
		t.Fatal(err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestSetup_Errors(t *testing.T) {
	ts, client, _ := newTestServer(t)
	// No setup yet

	token := fetchCSRFCookie(t, client, ts.URL+"/setup")

	// 1. Invalid JSON
	resp := postJSON(t, client, ts.URL+"/api/v1/setup", token, `{invalid`)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	// 2. Missing fields
	resp = postJSON(t, client, ts.URL+"/api/v1/setup", token, `{}`)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	jar, _ := cookiejar.New(nil)
	client.Jar = jar

	token := fetchCSRFCookie(t, client, ts.URL+"/login")
	resp := postJSON(t, client, ts.URL+"/api/v1/login", token, `{"email":"admin@test.com","password":"wrong"}`)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}
