package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/paleicikas/importinvoices/server/internal/service"
)

func TestExportTemplateHandlers(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	// 1. List templates
	resp, err := client.Get(ts.URL + "/settings/export-templates")
	if err != nil {
		t.Fatalf("GET /settings/export-templates: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// 2. Create template
	token := fetchCSRFCookie(t, client, ts.URL+"/settings/export-templates")
	resp, err = client.PostForm(ts.URL+"/settings/export-templates", url.Values{
		"title":       {"Test Template"},
		"type":        {"file"},
		"files[0].filename": {"test.json"},
		"files[0].content":  {"{}"},
		csrfFormField: {token},
	})
	if err != nil {
		t.Fatalf("POST /settings/export-templates: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}

	// 3. Get the ID of the created template
	// We'll just list them and take the first one that isn't system
	// Since we don't have easy access to svc here, we'll use the API endpoint
	resp, err = client.Get(ts.URL + "/api/v1/export/templates")
	if err != nil {
		t.Fatalf("GET /api/v1/export/templates: %v", err)
	}
	var templates []service.ExportTemplate
	if err := json.NewDecoder(resp.Body).Decode(&templates); err != nil {
		t.Fatalf("decode templates: %v", err)
	}
	resp.Body.Close()

	var tmplID string
	for _, t := range templates {
		if !t.IsSystem {
			tmplID = t.ID
			break
		}
	}
	if tmplID == "" {
		t.Fatal("no custom template found")
	}

	// 4. Edit template (GET)
	resp, err = client.Get(ts.URL + "/settings/export-templates/" + tmplID + "/edit")
	if err != nil {
		t.Fatalf("GET /settings/export-templates/%s/edit: %v", tmplID, err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// 5. Preview template (GET)
	resp, err = client.Get(ts.URL + "/settings/export-templates/" + tmplID)
	if err != nil {
		t.Fatalf("GET /settings/export-templates/%s: %v", tmplID, err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// 6. Update template (POST)
	resp, err = client.PostForm(ts.URL+"/settings/export-templates/"+tmplID, url.Values{
		"title":       {"Updated Template"},
		"files[0].filename": {"updated.json"},
		"files[0].content":  {"{\"updated\":true}"},
		csrfFormField: {token},
	})
	if err != nil {
		t.Fatalf("POST /settings/export-templates/%s: %v", tmplID, err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}

	// 7. Favorite template
	resp, err = client.PostForm(ts.URL+"/settings/export-templates/"+tmplID+"/favorite", url.Values{
		csrfFormField: {token},
	})
	if err != nil {
		t.Fatalf("POST /settings/export-templates/%s/favorite: %v", tmplID, err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}

	// 8. Delete template
	resp, err = client.PostForm(ts.URL+"/settings/export-templates/"+tmplID+"/delete", url.Values{
		csrfFormField: {token},
	})
	if err != nil {
		t.Fatalf("POST /settings/export-templates/%s/delete: %v", tmplID, err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}

	// 9. Delete system template (forbidden)
	resp, err = client.PostForm(ts.URL+"/settings/export-templates/system_generic/delete", url.Values{
		csrfFormField: {token},
	})
	if err != nil {
		t.Fatalf("POST /settings/export-templates/system_generic/delete: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}

	// 10. Preview API
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/export/templates/preview", strings.NewReader(`{
		"type": "file",
		"files": [{"filename": "test.json", "content": "{}"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeaderName, token)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/export/templates/preview: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestExportTemplateNewPage(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	resp, err := client.Get(ts.URL + "/settings/export-templates/new")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestExportTemplatePreviewPage(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	// Get a system template ID
	resp, err := client.Get(ts.URL + "/api/v1/export/templates")
	if err != nil {
		t.Fatal(err)
	}
	var templates []service.ExportTemplate
	if err := json.NewDecoder(resp.Body).Decode(&templates); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(templates) == 0 {
		t.Fatal("expected templates")
	}
	tmplID := templates[0].ID

	resp, err = client.Get(ts.URL + "/settings/export-templates/" + tmplID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestExportTemplateHandlers_Errors(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	token := fetchCSRFCookie(t, client, ts.URL+"/settings/export-templates")

	// 1. Create template (missing title)
	resp, err := client.PostForm(ts.URL+"/settings/export-templates", url.Values{
		"type":        {"file"},
		csrfFormField: {token},
	})
	if err != nil {
		t.Fatal(err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusFound { // Redirects back on error
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}

	// 2. Preview API (invalid JSON)
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/export/templates/preview", strings.NewReader(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeaderName, token)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
