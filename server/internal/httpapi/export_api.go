package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/paleicikas/importinvoices/server/internal/export"
	"github.com/paleicikas/importinvoices/server/internal/service"
)

func (s *Server) handleExportTemplatesAPI(w http.ResponseWriter, r *http.Request) {
	org, err := s.svc.GetOrganization(r.Context())
	if err != nil || org == nil {
		http.Error(w, "organization not found", http.StatusBadRequest)
		return
	}
	templates, err := s.svc.ListExportTemplates(r.Context(), org.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(templates); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type exportAPIRequest struct {
	IDs          []string `json:"ids"`
	Format       string   `json:"format"`
	TemplateID   string   `json:"template_id"`
	InvoiceType  string   `json:"invoice_type"`
	MarkExported bool     `json:"mark_exported"`
}

func (s *Server) handleExportAPI(w http.ResponseWriter, r *http.Request) {
	var req exportAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.IDs) == 0 {
		http.Error(w, "ids required", http.StatusBadRequest)
		return
	}

	baseURL := requestBaseURL(r)

	userID := ""
	if user, ok := userFromContext(r.Context()); ok {
		userID = user.ID
	}

	invoiceType := export.InvoiceType(req.InvoiceType)
	if invoiceType == "" {
		invoiceType = export.InvoiceTypePurchases
	}

	var buf bytes.Buffer
	result, err := s.svc.ExportInvoices(r.Context(), service.ExportParams{
		IDs:          req.IDs,
		Format:       req.Format,
		TemplateID:   req.TemplateID,
		InvoiceType:  invoiceType,
		MarkExported: req.MarkExported,
		BaseURL:      baseURL,
		UserID:       userID,
	}, &buf)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if result.IsAPI {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(result.APIResponse)); err != nil {
			return
		}
		return
	}

	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", result.Filename))
	if _, err := buf.WriteTo(w); err != nil {
		return
	}
}
