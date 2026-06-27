package httpapi

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/paleicikas/importinvoices/server/internal/export"
	"github.com/paleicikas/importinvoices/server/internal/service"
)

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ids := r.Form["ids"]
	if len(ids) == 0 {
		http.Error(w, "No invoices selected", http.StatusBadRequest)
		return
	}

	format := r.FormValue("format")
	templateID := r.FormValue("template_id")
	markExported := r.FormValue("mark_exported") == "1" || r.FormValue("mark_exported") == "true"
	invoiceType := export.InvoiceType(r.FormValue("invoice_type"))
	if invoiceType == "" {
		invoiceType = export.InvoiceTypePurchases
	}

	baseURL := requestBaseURL(r)

	user, _ := userFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	var buf bytes.Buffer
	result, err := s.svc.ExportInvoices(r.Context(), service.ExportParams{
		IDs:          ids,
		Format:       format,
		TemplateID:   templateID,
		InvoiceType:  invoiceType,
		MarkExported: markExported,
		BaseURL:      baseURL,
		UserID:       userID,
	}, &buf)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if result == nil {
		return
	}

	if result.IsAPI {
		if r.FormValue("redirect") == "1" {
			msg := "API export successful"
			if result.APIResponse != "" {
				msg = "API export successful: " + truncate(result.APIResponse, 200)
			}
			s.setFlash(w, msg, "success")
			http.Redirect(w, r, "/invoices?tab=export", http.StatusSeeOther)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(result.APIResponse)); err != nil {
			return
		}
		return
	}

	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", result.Filename))
	_, _ = buf.WriteTo(w)
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
