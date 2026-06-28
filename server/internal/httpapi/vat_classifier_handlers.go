package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/reqctx"
)

func (s *Server) handleVatClassifiersPage(w http.ResponseWriter, r *http.Request) {
	org, _ := reqctx.Organization(r.Context())
	classifiers, err := s.svc.ListVatClassifiers(r.Context(), org.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	availableCountries, _ := s.svc.ListAvailableCatalogCountries()

	s.render.RenderPage(w, r, "vat_classifiers.html", map[string]any{
		"Title":              "VAT Classifiers",
		"Page":               "settings",
		"ActiveTab":          "vat-classifiers",
		"Classifiers":        classifiers,
		"AvailableCountries": availableCountries,
	})
}

func (s *Server) handleVatClassifierNewPage(w http.ResponseWriter, r *http.Request) {
	availableCountries, _ := s.svc.ListAvailableCatalogCountries()

	s.render.RenderPage(w, r, "vat_classifier_edit.html", map[string]any{
		"Title":              "New VAT Classifier",
		"Page":               "settings",
		"ActiveTab":          "vat-classifiers",
		"IsNew":              true,
		"AvailableCountries": availableCountries,
		"Classifier": &domain.VatClassifier{
			Active:        true,
			IncludeInIsaf: true,
		},
	})
}

func (s *Server) handleVatClassifierCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	org, _ := reqctx.Organization(r.Context())
	tariff, _ := strconv.ParseFloat(r.FormValue("tariff"), 64)

	vc := &domain.VatClassifier{
		OrgID:           org.ID,
		Country:         strings.ToUpper(r.FormValue("country")),
		Code:            r.FormValue("code"),
		Tariff:          tariff,
		Description:     ptr(r.FormValue("description")),
		Example:         ptr(r.FormValue("example")),
		ReceivingRule:   ptr(r.FormValue("receiving_rule")),
		IssuedRule:      ptr(r.FormValue("issued_rule")),
		Active:          r.FormValue("active") == "on",
		ReverseCharge:   r.FormValue("reverse_charge") == "on",
		PurchaseAccount: ptr(r.FormValue("purchase_account")),
		IncludeInIsaf:   r.FormValue("include_in_isaf") == "on",
	}

	if err := s.svc.CreateVatClassifier(r.Context(), vc); err != nil {
		s.setFlash(w, err.Error(), "error")
		s.render.RenderPage(w, r, "vat_classifier_edit.html", map[string]any{
			"Title":      "New VAT Classifier",
			"Page":       "settings",
			"ActiveTab":  "vat-classifiers",
			"IsNew":      true,
			"Classifier": vc,
		})
		return
	}

	s.setFlash(w, "VAT classifier created", "success")
	http.Redirect(w, r, "/settings/vat-classifiers", http.StatusSeeOther)
}

func (s *Server) handleVatClassifierEditPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	vc, err := s.svc.GetVatClassifier(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	org, _ := reqctx.Organization(r.Context())
	if vc.OrgID != org.ID {
		http.NotFound(w, r)
		return
	}

	availableCountries, _ := s.svc.ListAvailableCatalogCountries()

	s.render.RenderPage(w, r, "vat_classifier_edit.html", map[string]any{
		"Title":              "Edit VAT Classifier",
		"Page":               "settings",
		"ActiveTab":          "vat-classifiers",
		"IsNew":              false,
		"AvailableCountries": availableCountries,
		"Classifier":         vc,
	})
}

func (s *Server) handleVatClassifierUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	org, _ := reqctx.Organization(r.Context())
	tariff, _ := strconv.ParseFloat(r.FormValue("tariff"), 64)

	vc := &domain.VatClassifier{
		ID:              id,
		OrgID:           org.ID,
		Country:         strings.ToUpper(r.FormValue("country")),
		Code:            r.FormValue("code"),
		Tariff:          tariff,
		Description:     ptr(r.FormValue("description")),
		Example:         ptr(r.FormValue("example")),
		ReceivingRule:   ptr(r.FormValue("receiving_rule")),
		IssuedRule:      ptr(r.FormValue("issued_rule")),
		Active:          r.FormValue("active") == "on",
		ReverseCharge:   r.FormValue("reverse_charge") == "on",
		PurchaseAccount: ptr(r.FormValue("purchase_account")),
		IncludeInIsaf:   r.FormValue("include_in_isaf") == "on",
	}

	if err := s.svc.UpdateVatClassifier(r.Context(), vc); err != nil {
		s.setFlash(w, err.Error(), "error")
		s.render.RenderPage(w, r, "vat_classifier_edit.html", map[string]any{
			"Title":      "Edit VAT Classifier",
			"Page":       "settings",
			"ActiveTab":  "vat-classifiers",
			"IsNew":      false,
			"Classifier": vc,
		})
		return
	}

	s.setFlash(w, "VAT classifier updated", "success")
	http.Redirect(w, r, "/settings/vat-classifiers", http.StatusSeeOther)
}

func (s *Server) handleVatClassifierDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	org, _ := reqctx.Organization(r.Context())

	if err := s.svc.DeleteVatClassifier(r.Context(), id, org.ID); err != nil {
		s.setFlash(w, err.Error(), "error")
	} else {
		s.setFlash(w, "VAT classifier deleted", "success")
	}

	http.Redirect(w, r, "/settings/vat-classifiers", http.StatusSeeOther)
}

func (s *Server) handleVatClassifierImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	org, _ := reqctx.Organization(r.Context())
	country := r.FormValue("country")
	mode := r.FormValue("mode") // "all" or "missing"

	if err := s.svc.ImportCatalogCountry(r.Context(), org.ID, country, mode == "missing"); err != nil {
		s.setFlash(w, err.Error(), "error")
	} else {
		s.setFlash(w, fmt.Sprintf("Imported classifiers for %s", country), "success")
	}

	http.Redirect(w, r, "/settings/vat-classifiers", http.StatusSeeOther)
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
