package webui

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderStandalonePageLogin(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	req := httptest.NewRequest("GET", "/login", nil)
	rec := httptest.NewRecorder()

	r.RenderStandalonePage(rec, req, "login", nil)

	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %q", rec.Code, body)
	}
	for _, want := range []string{"loginForm", "Sign In", "lang=en"} {
		if !strings.Contains(body, want) {
			t.Fatalf("login page missing %q", want)
		}
	}
}

func TestRenderStandalonePageSetup(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	req := httptest.NewRequest("GET", "/setup", nil)
	rec := httptest.NewRecorder()

	r.RenderStandalonePage(rec, req, "setup", nil)

	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %q", rec.Code, body)
	}
	for _, want := range []string{"setupForm", "Complete Setup", "lang=lt"} {
		if !strings.Contains(body, want) {
			t.Fatalf("setup page missing %q", want)
		}
	}
}
