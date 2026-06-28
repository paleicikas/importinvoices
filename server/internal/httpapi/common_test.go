package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/media"
	"github.com/paleicikas/importinvoices/server/internal/reqctx"
	"github.com/paleicikas/importinvoices/server/internal/service"
	"github.com/paleicikas/importinvoices/server/internal/storage"
	"github.com/paleicikas/importinvoices/server/internal/webui"
)

func newTestServer(t *testing.T) (*httptest.Server, *http.Client, *Server) {
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
	if err := svc.SeedExportTemplates(context.Background()); err != nil {
		t.Fatalf("seed: %v", err)
	}
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

	return ts, client, srv
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
	defer resp.Body.Close()
	
	// Try to get from response cookies first
	for _, c := range resp.Cookies() {
		if c.Name == csrfCookieName {
			return c.Value
		}
	}

	// Fallback to jar
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

func postForm(t *testing.T, client *http.Client, targetURL string, values url.Values) *http.Response {
	t.Helper()
	resp, err := client.PostForm(targetURL, values)
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func createTestInvoice(t *testing.T, s *Server) string {
	ctx := context.Background()
	user, err := s.svc.Authenticate(ctx, "admin@test.com", "secret123")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	org, _ := s.svc.GetOrganization(ctx)
	ctx = reqctx.WithUser(ctx, user)
	ctx = reqctx.WithOrganization(ctx, org)

	inv := &domain.Invoice{
		ID:          uuid.New().String(),
		UserID:      user.ID,
		OrgID:       org.ID,
		Status:      "processed",
		Filename:    "test.pdf",
		StoragePath: "test.pdf",
		Checksum:    "test-checksum",
	}
	if err := s.svc.CreateInvoice(ctx, inv); err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	return inv.ID
}
