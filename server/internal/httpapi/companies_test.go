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
	if _, err := srv.svc.UpsertCompany(ctx, company2, nil); err != nil {
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
	if _, err := srv.svc.UpsertCompany(ctx, company, nil); err != nil {
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

func TestCompanySearchAPI(t *testing.T) {
	ts, client, srv := newTestServer(t)
	setupAndLogin(t, ts, client)

	ctx := context.Background()

	var orgID string
	if err := srv.svc.Store().DB().QueryRow("SELECT id FROM organizations WHERE title = ?", "Test Org").Scan(&orgID); err != nil {
		t.Fatalf("find test org: %v", err)
	}

	vat := "LT999"
	alpha := domain.Company{OrgID: orgID, Title: "Alpha Merge Target", VATCode: &vat}
	beta := domain.Company{OrgID: orgID, Title: "Beta Company"}
	if _, err := srv.svc.UpsertCompany(ctx, alpha, nil); err != nil {
		t.Fatalf("upsert alpha: %v", err)
	}
	if _, err := srv.svc.UpsertCompany(ctx, beta, nil); err != nil {
		t.Fatalf("upsert beta: %v", err)
	}

	all, err := srv.svc.ListCompanies(ctx, orgID, service.CompanyListParams{})
	if err != nil {
		t.Fatalf("list companies: %v", err)
	}
	var alphaID, betaID string
	for _, c := range all {
		if c.Title == "Alpha Merge Target" {
			alphaID = c.ID
		}
		if c.Title == "Beta Company" {
			betaID = c.ID
		}
	}
	if alphaID == "" || betaID == "" {
		t.Fatalf("expected to find alpha and beta, got %d companies", len(all))
	}

	// Search by title fragment returns Alpha (excluding beta to prove exclude works).
	resp, err := client.Get(ts.URL + "/api/v1/companies/search?q=Alpha&exclude=" + betaID)
	if err != nil {
		t.Fatalf("GET search: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Alpha Merge Target") {
		t.Errorf("expected Alpha in response, got %s", body)
	}
	if strings.Contains(string(body), betaID) {
		t.Errorf("exclude= did not filter out beta, got %s", body)
	}

	// Searching with exclude=alphaID must hide Alpha even though it matches.
	resp, err = client.Get(ts.URL + "/api/v1/companies/search?q=Alpha&exclude=" + alphaID)
	if err != nil {
		t.Fatalf("GET search exclude-alpha: %v", err)
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	if strings.Contains(string(body), "Alpha Merge Target") {
		t.Errorf("exclude= did not filter out the source company, got %s", body)
	}

	// Search by normalized VAT code returns Alpha (UpsertCompany strips the
	// leading "LT" country prefix, so the stored value is "999").
	resp, err = client.Get(ts.URL + "/api/v1/companies/search?q=999&exclude=" + betaID)
	if err != nil {
		t.Fatalf("GET search by vat: %v", err)
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Alpha Merge Target") {
		t.Errorf("expected Alpha by VAT code, got %s", body)
	}

	// No match returns an empty JSON array, not null.
	resp, err = client.Get(ts.URL + "/api/v1/companies/search?q=ZZZNoMatch&exclude=")
	if err != nil {
		t.Fatalf("GET search no-match: %v", err)
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "[]" {
		t.Errorf("expected empty array, got %s", body)
	}
}
