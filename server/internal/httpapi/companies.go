package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"github.com/go-chi/chi/v5"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/reqctx"
	"github.com/paleicikas/importinvoices/server/internal/service"
)

func (s *Server) handleCompanies(w http.ResponseWriter, r *http.Request) {
	org, _ := reqctx.Organization(r.Context())

	search := r.URL.Query().Get("q")

	// Parse column filters
	columnFilters := make(map[int][]string)
	for k, v := range r.URL.Query() {
		if strings.HasPrefix(k, "f.") {
			var col int
			if _, err := fmt.Sscanf(k, "f.%d", &col); err == nil {
				columnFilters[col] = v
			}
		}
	}

	sortCol := 0
	if v := r.URL.Query().Get("sort"); v != "" {
		sortCol, _ = strconv.Atoi(v)
	}
	sortDir := r.URL.Query().Get("dir")
	if sortDir == "" {
		sortDir = "asc"
	}

	companies, err := s.svc.ListCompanies(r.Context(), org.ID, service.CompanyListParams{
		Search:        search,
		ColumnFilters: columnFilters,
		SortCol:       sortCol,
		SortDir:       sortDir,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render.RenderPage(w, r, "companies.html", map[string]any{
		"Title":         "Companies",
		"Page":          "companies",
		"ListURL":       "/companies",
		"Companies":     companies,
		"SortCol":       sortCol,
		"SortDir":       sortDir,
		"Search":        search,
		"ColumnFilters": columnFilters,
		"Tab":           "",
	})
}

func (s *Server) handleCompanyDetails(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	company, err := s.svc.GetCompanyForOrg(r.Context(), id)
	if err != nil {
		http.Error(w, "Company not found", http.StatusNotFound)
		return
	}

	tab := r.URL.Query().Get("tab")
	if tab == "" || tab == "banks" {
		tab = "details"
	}

	search := r.URL.Query().Get("q")

	// Parse column filters
	columnFilters := make(map[int][]string)
	for k, v := range r.URL.Query() {
		if strings.HasPrefix(k, "f.") {
			var col int
			if _, err := fmt.Sscanf(k, "f.%d", &col); err == nil {
				columnFilters[col] = v
			}
		}
	}

	sortCol := 0
	if v := r.URL.Query().Get("sort"); v != "" {
		sortCol, _ = strconv.Atoi(v)
	}
	sortDir := r.URL.Query().Get("dir")
	if sortDir == "" {
		sortDir = "desc"
	}

	page := 1
	if v := r.URL.Query().Get("page"); v != "" {
		page, _ = strconv.Atoi(v)
	}
	if page < 1 {
		page = 1
	}
	limit := 20
	offset := (page - 1) * limit

	var purchases []domain.Invoice
	var sales []domain.Invoice
	var banks []any

	if company.Banks != nil && *company.Banks != "" && *company.Banks != "null" {
		_ = json.Unmarshal([]byte(*company.Banks), &banks)
	}

	if tab == "purchases" {
		purchases, _, _ = s.svc.ListInvoicesByCompany(r.Context(), company, false, service.InvoiceListParams{
			Search:        search,
			ColumnFilters: columnFilters,
			SortCol:       sortCol,
			SortDir:       sortDir,
			Limit:         limit,
			Offset:        offset,
		})
	} else if tab == "sales" {
		sales, _, _ = s.svc.ListInvoicesByCompany(r.Context(), company, true, service.InvoiceListParams{
			Search:        search,
			ColumnFilters: columnFilters,
			SortCol:       sortCol,
			SortDir:       sortDir,
			Limit:         limit,
			Offset:        offset,
		})
	}

	// Merge targets are no longer preloaded here; the merge form loads them
	// on demand via /api/v1/companies/search so that organizations with a
	// very large number of companies do not render every row into the page.

	s.render.RenderPage(w, r, "company_details.html", map[string]any{
		"Title":         company.Title,
		"Page":          "companies",
		"ListURL":       fmt.Sprintf("/companies/%s", id),
		"Company":       company,
		"Tab":           tab,
		"Search":        search,
		"ColumnFilters": columnFilters,
		"SortCol":       sortCol,
		"SortDir":       sortDir,
		"Purchases":     purchases,
		"Sales":         sales,
		"Banks":         banks,
		"CurrentPage":   page,
		"Limit":         limit,
	})
}

func (s *Server) handleCompanyDelete(w http.ResponseWriter, r *http.Request) {
	org, _ := reqctx.Organization(r.Context())
	id := chi.URLParam(r, "id")

	err := s.svc.DeleteCompany(r.Context(), org.ID, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Company not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, service.ErrCompanyHasInvoices) {
			s.setFlash(w, r, "Cannot delete a company with linked invoices", "error")
			http.Redirect(w, r, "/companies", http.StatusSeeOther)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.setFlash(w, r, "Company deleted successfully", "success")
	http.Redirect(w, r, "/companies", http.StatusSeeOther)
}

func (s *Server) handleCompanyMerge(w http.ResponseWriter, r *http.Request) {
	org, _ := reqctx.Organization(r.Context())
	sourceID := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	targetID := r.FormValue("target_id")
	if targetID == "" {
		s.setFlash(w, r, "Select a company to merge into", "error")
		http.Redirect(w, r, "/companies/"+sourceID, http.StatusSeeOther)
		return
	}
	if err := s.svc.MergeCompanies(r.Context(), org.ID, sourceID, targetID); err != nil {
		s.setFlash(w, r, "Merge failed: "+err.Error(), "error")
		http.Redirect(w, r, "/companies/"+sourceID, http.StatusSeeOther)
		return
	}
	s.setFlash(w, r, "Companies merged successfully", "success")
	http.Redirect(w, r, "/companies/"+targetID, http.StatusSeeOther)
}

// handleCompanySearchAPI returns a lightweight JSON list of companies in the
// current organization matching the ?q= search term, excluding the company
// identified by ?exclude= (typically the company currently being viewed, so it
// cannot be picked as a merge target). It is org-scoped via authMiddleware and
// capped at a small limit so it is safe to call from a typeahead even when the
// organization has millions of companies. CSRF is not required (GET only).
func (s *Server) handleCompanySearchAPI(w http.ResponseWriter, r *http.Request) {
	org, _ := reqctx.Organization(r.Context())
	if org == nil {
		http.Error(w, "organization not found", http.StatusBadRequest)
		return
	}
	excludeID := r.URL.Query().Get("exclude")
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	companies, err := s.svc.SearchCompaniesForMerge(r.Context(), org.ID, excludeID, search, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type companyHit struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		VATCode string `json:"vat_code,omitempty"`
	}
	hits := make([]companyHit, 0, len(companies))
	for _, c := range companies {
		vat := ""
		if c.VATCode != nil {
			vat = *c.VATCode
		}
		hits = append(hits, companyHit{ID: c.ID, Title: c.Title, VATCode: vat})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(hits); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
