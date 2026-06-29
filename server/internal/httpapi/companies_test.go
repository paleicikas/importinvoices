package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/service"
)

func TestCompaniesHandlers(t *testing.T) {
	ts, client, srv := newTestServer(t)
	setupAndLogin(t, ts, client)

	// 1. List companies
	resp, err := client.Get(ts.URL + "/companies")
	if err != nil {
		t.Fatalf("GET /companies: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "Companies") {
		t.Error("missing Companies title")
	}

	// 2. Company details (not found)
	resp, err = client.Get(ts.URL + "/companies/missing")
	if err != nil {
		t.Fatalf("GET /companies/missing: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}

	// 3. Delete company (non-existent)
	token := fetchCSRFCookie(t, client, ts.URL+"/companies")
	resp, err = client.PostForm(ts.URL+"/companies/missing/delete", url.Values{
		csrfFormField: {token},
	})
	if err != nil {
		t.Fatalf("POST /companies/missing/delete: %v", err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}

	// 4. Delete company (success)
	// Find the organization created during setup
	var orgID, orgTitle string
	err = srv.svc.Store().DB().QueryRow("SELECT id, title FROM organizations WHERE title = ?", "Test Org").Scan(&orgID, &orgTitle)
	if err != nil {
		t.Fatalf("find test org: %v", err)
	}

	ctx := context.Background()
	company2 := domain.Company{
		OrgID: orgID,
		Title: "To Delete",
	}
	if err := srv.svc.UpsertCompany(ctx, company2, nil); err != nil {
		t.Fatalf("upsert company: %v", err)
	}

	companies, err := srv.svc.ListCompanies(ctx, orgID, service.CompanyListParams{Search: "To Delete"})
	if err != nil {
		t.Fatalf("list companies: %v", err)
	}
	if len(companies) == 0 {
		t.Fatal("expected company 'To Delete' to delete")
	}
	toDeleteID := companies[0].ID

	resp, err = client.PostForm(ts.URL+"/companies/"+toDeleteID+"/delete", url.Values{
		csrfFormField: {token},
	})
	if err != nil {
		t.Fatalf("POST /companies/%s/delete: %v", toDeleteID, err)
	}
	discardResponseBody(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}
}

func TestCompanyDetails(t *testing.T) {
	ts, client, srv := newTestServer(t)
	setupAndLogin(t, ts, client)

	// Create a company
	ctx := context.Background()
	
	// Find the organization created during setup
	var orgID, orgTitle string
	err := srv.svc.Store().DB().QueryRow("SELECT id, title FROM organizations WHERE title = ?", "Test Org").Scan(&orgID, &orgTitle)
	if err != nil {
		t.Fatalf("find test org: %v", err)
	}

	code := "123"
	vat := "LT123"
	company := domain.Company{
		OrgID:   orgID,
		Title:   "Test Company",
		Code:    &code,
		VATCode: &vat,
	}
	if err := srv.svc.UpsertCompany(ctx, company, nil); err != nil {
		t.Fatalf("upsert company: %v", err)
	}

	// Get the company ID
	companies, err := srv.svc.ListCompanies(ctx, orgID, service.CompanyListParams{})
	if err != nil {
		t.Fatalf("list companies: %v", err)
	}
	if len(companies) == 0 {
		t.Fatalf("expected 1 company, got 0 (orgID=%s)", orgID)
	}
	companyID := companies[0].ID

	resp, err := client.Get(ts.URL + "/companies/" + companyID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// Test non-existent company
	resp, err = client.Get(ts.URL + "/companies/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
