package httpapi

import (
	"net/http"
	"testing"
	"net/url"
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

	// 5. Change password
	resp, err = client.PostForm(ts.URL+"/profile", url.Values{
		"password":        {"newpass123"},
		"password_repeat": {"newpass123"},
		csrfFormField:     {token},
	})
	if err != nil {
		t.Fatalf("POST /profile (password): %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
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
