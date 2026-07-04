package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestSettingsHandlers(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	// 1. GET settings
	resp, err := client.Get(ts.URL + "/settings")
	if err != nil {
		t.Fatalf("GET /settings: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}

	// 2. POST settings
	token := fetchCSRFCookie(t, client, ts.URL+"/settings")
	resp, err = client.PostForm(ts.URL+"/settings", url.Values{
		"llm_provider":   {"openai"},
		"openai_api_key": {"sk-test"},
		"org_title":      {"Updated Org"},
		csrfFormField:   {token},
	})
	if err != nil {
		t.Fatalf("POST /settings: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}

	// 3. GET profile
	resp, err = client.Get(ts.URL + "/profile")
	if err != nil {
		t.Fatalf("GET /profile: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}

	// 4. Update profile
	resp, err = client.PostForm(ts.URL+"/profile", url.Values{
		"name":        {"New Name"},
		"email":       {"admin@example.com"},
		csrfFormField: {token},
	})
	if err != nil {
		t.Fatalf("POST /profile: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}

	// 5. Change password (requires current password)
	resp, err = client.PostForm(ts.URL+"/profile", url.Values{
		"current_password": {"secret123"},
		"password":         {"newpass123"},
		"password_repeat":  {"newpass123"},
		csrfFormField:      {token},
	})
	if err != nil {
		t.Fatalf("POST /profile (password): %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}
}

// TestT14_PasswordChangeRequiresCurrent verifies P1-7: changing the password
// without the current password is rejected and the password is NOT changed.
func TestT14_PasswordChangeRequiresCurrent(t *testing.T) {
	ts, client, srv := newTestServer(t)
	setupAndLogin(t, ts, client)
	token := fetchCSRFCookie(t, client, ts.URL+"/profile")

	// Attempt to change password without current_password.
	resp, err := client.PostForm(ts.URL+"/profile", url.Values{
		"password":        {"brand-new-pw-123"},
		"password_repeat": {"brand-new-pw-123"},
		csrfFormField:     {token},
	})
	if err != nil {
		t.Fatal(err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}

	// The new password must NOT authenticate (change was rejected).
	if _, err := srv.svc.Authenticate(context.Background(), "admin@test.com", "brand-new-pw-123"); err == nil {
		t.Error("new password accepted without current_password: expected rejection")
	}
	// The old password still works.
	if _, err := srv.svc.Authenticate(context.Background(), "admin@test.com", "secret123"); err != nil {
		t.Errorf("old password no longer works: %v", err)
	}

	// Wrong current password is also rejected.
	resp, err = client.PostForm(ts.URL+"/profile", url.Values{
		"current_password": {"wrong-current-pw"},
		"password":         {"brand-new-pw-123"},
		"password_repeat":  {"brand-new-pw-123"},
		csrfFormField:      {token},
	})
	if err != nil {
		t.Fatal(err)
	}
	discardResponseBody(t, resp)
	if _, err := srv.svc.Authenticate(context.Background(), "admin@test.com", "brand-new-pw-123"); err == nil {
		t.Error("new password accepted with wrong current_password: expected rejection")
	}

	// Correct current password succeeds.
	resp, err = client.PostForm(ts.URL+"/profile", url.Values{
		"name":             {"Admin"},
		"email":            {"admin@test.com"},
		"current_password": {"secret123"},
		"password":         {"brand-new-pw-123"},
		"password_repeat":  {"brand-new-pw-123"},
		csrfFormField:      {token},
	})
	if err != nil {
		t.Fatal(err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}
	if _, err := srv.svc.Authenticate(context.Background(), "admin@test.com", "brand-new-pw-123"); err != nil {
		t.Errorf("new password should authenticate after correct change: %v", err)
	}
}

func TestSettingsHandlers_Errors(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	token := fetchCSRFCookie(t, client, ts.URL+"/settings")

	// 1. POST settings (missing org_title)
	resp, err := client.PostForm(ts.URL+"/settings", url.Values{
		"llm_provider": {"openai"},
		csrfFormField:  {token},
	})
	if err != nil {
		t.Fatal(err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}
}

func TestProfileHandlers_Errors(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	token := fetchCSRFCookie(t, client, ts.URL+"/profile")

	// Test password mismatch
	resp := postForm(t, client, ts.URL+"/profile", url.Values{
		csrfFormField:     {token},
		"password":        {"newpass"},
		"password_repeat": {"mismatch"},
	})
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}
}
