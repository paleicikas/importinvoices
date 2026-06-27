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
	company, err := s.svc.GetCompany(r.Context(), id)
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
			s.setFlash(w, "Cannot delete a company with linked invoices", "error")
			http.Redirect(w, r, "/companies", http.StatusSeeOther)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.setFlash(w, "Company deleted successfully", "success")
	http.Redirect(w, r, "/companies", http.StatusSeeOther)
}
