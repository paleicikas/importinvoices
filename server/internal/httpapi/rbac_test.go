package httpapi

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/paleicikas/importinvoices/server/internal/domain"
)

// loginAs creates a fresh authenticated client for the given credentials.
func loginAs(t *testing.T, ts *httptest.Server, email, password string) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	c := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	token := fetchCSRFCookie(t, c, ts.URL+"/login")
	resp := postJSON(t, c, ts.URL+"/api/v1/login", token, `{"email":"`+email+`","password":"`+password+`"}`)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login %s: status %d", email, resp.StatusCode)
	}
	return c
}

// createOperator creates an operator user directly via the service and returns
// the credentials. The admin (already set up) performs the creation.
func createOperator(t *testing.T, srv *Server) string {
	t.Helper()
	_, err := srv.svc.CreateUserWithRole(context.Background(), "op@test.com", "secret123", "Operator", domain.RoleOperator)
	if err != nil {
		t.Fatalf("create operator: %v", err)
	}
	return "op@test.com"
}

func TestRBAC_OperatorBlockedFromSettings(t *testing.T) {
	ts, adminClient, srv := newTestServer(t)
	setupAndLogin(t, ts, adminClient)
	createOperator(t, srv)

	op := loginAs(t, ts, "op@test.com", "secret123")

	// Operator cannot GET the LLM settings page -> redirect (not 200).
	resp, err := op.Get(ts.URL + "/settings/llm")
	if err != nil {
		t.Fatalf("GET /settings/llm: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("GET /settings/llm: status %d, want 303 (redirect)", resp.StatusCode)
	}

	// Operator cannot POST settings -> 403.
	csrf := csrfTokenFromJar(op, ts.URL)
	form := url.Values{
		"csrf_token":   {csrf},
		"llm_provider": {"openai"},
	}
	resp = postForm(t, op, ts.URL+"/settings/llm", form)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("POST /settings/llm: status %d, want 403", resp.StatusCode)
	}
}

func TestRBAC_OperatorCanWork(t *testing.T) {
	ts, adminClient, srv := newTestServer(t)
	setupAndLogin(t, ts, adminClient)
	createOperator(t, srv)

	op := loginAs(t, ts, "op@test.com", "secret123")

	for _, path := range []string{"/invoices", "/upload", "/companies", "/settings/vat-classifiers", "/settings/export-templates"} {
		resp, err := op.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		discardResponseBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status %d, want 200 (operator allowed)", path, resp.StatusCode)
		}
	}
}

func TestRBAC_OperatorBlockedFromVatTemplatesMutations(t *testing.T) {
	ts, adminClient, srv := newTestServer(t)
	setupAndLogin(t, ts, adminClient)
	createOperator(t, srv)

	op := loginAs(t, ts, "op@test.com", "secret123")
	csrf := csrfTokenFromJar(op, ts.URL)

	for _, path := range []string{"/settings/vat-classifiers", "/settings/export-templates"} {
		form := url.Values{
			"csrf_token": {csrf},
			"title":      {"x"},
			"code":       {"x"},
			"country":    {"LT"},
			"tariff":     {"21"},
		}
		resp := postForm(t, op, ts.URL+path, form)
		discardResponseBody(t, resp)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("POST %s: status %d, want 403", path, resp.StatusCode)
		}
	}
}

func TestRBAC_AdminCanManageUsers(t *testing.T) {
	ts, adminClient, srv := newTestServer(t)
	setupAndLogin(t, ts, adminClient)

	// Admin can view the users page.
	resp, err := adminClient.Get(ts.URL + "/settings/users")
	if err != nil {
		t.Fatalf("GET /settings/users: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /settings/users: status %d, want 200", resp.StatusCode)
	}

	// Admin creates an operator via the UI form.
	csrf := csrfTokenFromJar(adminClient, ts.URL)
	form := url.Values{
		"csrf_token": {csrf},
		"email":      {"newop@test.com"},
		"password":   {"secret123"},
		"name":       {"New Op"},
		"role":       {"operator"},
	}
	resp = postForm(t, adminClient, ts.URL+"/settings/users", form)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /settings/users: status %d, want 303", resp.StatusCode)
	}

	users, err := srv.svc.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	var newOpID string
	for _, u := range users {
		if u.Email == "newop@test.com" {
			if u.Role != domain.RoleOperator {
				t.Errorf("new op role = %q, want operator", u.Role)
			}
			newOpID = u.ID
		}
	}
	if newOpID == "" {
		t.Fatal("new operator not found in ListUsers")
	}

	// Promote the operator to admin via the role-change form.
	form = url.Values{
		"csrf_token": {csrf},
		"role":       {"admin"},
	}
	resp = postForm(t, adminClient, ts.URL+"/settings/users/"+newOpID+"/role", form)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST role-change: status %d, want 303", resp.StatusCode)
	}
	got, _ := srv.svc.GetUser(context.Background(), newOpID)
	if got.Role != domain.RoleAdmin {
		t.Errorf("after promote role = %q, want admin", got.Role)
	}

	// Delete the new admin (now there are 2 admins, allowed).
	form = url.Values{"csrf_token": {csrf}}
	resp = postForm(t, adminClient, ts.URL+"/settings/users/"+newOpID+"/delete", form)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST delete: status %d, want 303", resp.StatusCode)
	}
	if _, err := srv.svc.GetUser(context.Background(), newOpID); err == nil {
		t.Error("deleted user still retrievable")
	}
}

func TestRBAC_LastAdminProtection(t *testing.T) {
	ts, adminClient, srv := newTestServer(t)
	setupAndLogin(t, ts, adminClient)

	admin, err := srv.svc.Authenticate(context.Background(), "admin@test.com", "secret123")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	csrf := csrfTokenFromJar(adminClient, ts.URL)

	// Admin cannot delete themselves.
	form := url.Values{"csrf_token": {csrf}}
	resp := postForm(t, adminClient, ts.URL+"/settings/users/"+admin.ID+"/delete", form)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("self-delete: status %d, want 303", resp.StatusCode)
	}
	if _, err := srv.svc.GetUser(context.Background(), admin.ID); err != nil {
		t.Error("admin was deleted despite self-delete guard")
	}

	// Admin cannot demote themselves.
	form = url.Values{"csrf_token": {csrf}, "role": {"operator"}}
	resp = postForm(t, adminClient, ts.URL+"/settings/users/"+admin.ID+"/role", form)
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("self-demote: status %d, want 303", resp.StatusCode)
	}
	got, _ := srv.svc.GetUser(context.Background(), admin.ID)
	if got.Role != domain.RoleAdmin {
		t.Error("admin was demoted despite self-demote guard")
	}
}
