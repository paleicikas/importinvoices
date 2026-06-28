package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/reqctx"
)

func (s *Server) handleReviewStart(w http.ResponseWriter, r *http.Request) {
	id, err := s.svc.GetFirstUnconfirmedInvoiceID(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		s.setFlash(w, "No invoices waiting for confirmation", "info")
		http.Redirect(w, r, "/invoices?tab=ready", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/invoices/"+id, http.StatusSeeOther)
}

func (s *Server) handleReviewPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	inv, err := s.svc.GetInvoiceForOrg(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	items, err := s.svc.ListInvoiceItems(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var nextID, prevID string
	var currentIndex, totalCount int
	if inv.Status == "processed" {
		nextID, prevID, currentIndex, totalCount, _ = s.svc.GetInvoiceReviewQueue(r.Context(), id, inv.CreatedAt)
	}

	var duplicateOf *domain.Invoice
	var duplicateOfItems []domain.InvoiceItem
	if inv.DuplicateOfID != nil && *inv.DuplicateOfID != "" {
		orig, err := s.svc.GetInvoiceForOrg(r.Context(), *inv.DuplicateOfID)
		if err == nil {
			duplicateOf = orig
			duplicateOfItems, _ = s.svc.ListInvoiceItems(r.Context(), *inv.DuplicateOfID)
		}
	}

	org, _ := reqctx.Organization(r.Context())
	vatClassifiers, _ := s.svc.ListVatClassifiers(r.Context(), org.ID)
	availableCountries, _ := s.svc.ListAvailableCatalogCountries()

	s.render.RenderPage(w, r, "review.html", map[string]any{
		"Title":              "Review Invoice",
		"Page":               "invoices",
		"Invoice":            inv,
		"Items":              items,
		"NextID":             nextID,
		"PrevID":             prevID,
		"CurrentIndex":       currentIndex,
		"TotalCount":         totalCount,
		"DuplicateOf":        duplicateOf,
		"DuplicateOfItems":   duplicateOfItems,
		"VatClassifiers":     vatClassifiers,
		"AvailableCountries": availableCountries,
	})
}

func (s *Server) handleUpdateInvoice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	inv, err := s.svc.GetInvoiceForOrg(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if inv.Status == "pending" || inv.Status == "processing" {
		s.setFlash(w, "Invoice is currently being processed and cannot be updated", "error")
		http.Redirect(w, r, "/invoices/"+id, http.StatusSeeOther)
		return
	}

	if inv.Status == "duplicate" {
		s.setFlash(w, "Duplicate invoices cannot be updated", "error")
		http.Redirect(w, r, "/invoices/"+id, http.StatusSeeOther)
		return
	}

	// Helper to parse float
	parseFloat := func(s string) *float64 {
		if s == "" {
			return nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil
		}
		return &f
	}

	// Helper to parse date
	parseDate := func(s string) *time.Time {
		if s == "" {
			return nil
		}
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return nil
		}
		return &t
	}

	// Update invoice fields
	seriesAndNumber := r.FormValue("series_and_number")
	inv.SeriesAndNumber = &seriesAndNumber
	inv.IssueDate = parseDate(r.FormValue("issue_date"))
	inv.SupplyDate = parseDate(r.FormValue("supply_date"))
	inv.PaymentDueDate = parseDate(r.FormValue("payment_due_date"))
	currency := r.FormValue("currency")
	inv.Currency = &currency
	inv.AmountWithoutVat = parseFloat(r.FormValue("amount_without_vat"))
	inv.VatAmount = parseFloat(r.FormValue("vat_amount"))
	inv.AmountWithVat = parseFloat(r.FormValue("amount_with_vat"))

	sellerName := r.FormValue("seller_name")
	inv.SellerName = &sellerName
	sellerCode := r.FormValue("seller_code")
	inv.SellerCode = &sellerCode
	sellerVAT := r.FormValue("seller_vat")
	inv.SellerVAT = &sellerVAT
	sellerStreet := r.FormValue("seller_street")
	inv.SellerStreet = &sellerStreet
	sellerCity := r.FormValue("seller_city")
	inv.SellerCity = &sellerCity
	sellerCountry := r.FormValue("seller_country")
	inv.SellerCountry = &sellerCountry

	buyerName := r.FormValue("buyer_name")
	inv.BuyerName = &buyerName
	buyerCode := r.FormValue("buyer_code")
	inv.BuyerCode = &buyerCode
	buyerVAT := r.FormValue("buyer_vat")
	inv.BuyerVAT = &buyerVAT
	buyerStreet := r.FormValue("buyer_street")
	inv.BuyerStreet = &buyerStreet
	buyerCity := r.FormValue("buyer_city")
	inv.BuyerCity = &buyerCity
	buyerCountry := r.FormValue("buyer_country")
	inv.BuyerCountry = &buyerCountry

	// Parse items
	var items []domain.InvoiceItem
	for i := 0; ; i++ {
		desc := r.FormValue(fmt.Sprintf("items[%d].description", i))
		if desc == "" && r.FormValue(fmt.Sprintf("items[%d].total_price", i)) == "" {
			if i > 100 { // Safety break
				break
			}
			if i > 0 {
				break
			}
			if i == 0 && desc == "" && r.FormValue("items[0].total_price") == "" {
				break
			}
		}

		itemID := r.FormValue(fmt.Sprintf("items[%d].id", i))
		if itemID == "" {
			itemID = uuid.New().String()
		}

		vatClassifier := r.FormValue(fmt.Sprintf("items[%d].vat_classifier", i))

		items = append(items, domain.InvoiceItem{
			ID:            itemID,
			InvoiceID:     inv.ID,
			Description:   &desc,
			Quantity:      parseFloat(r.FormValue(fmt.Sprintf("items[%d].quantity", i))),
			UnitPrice:     parseFloat(r.FormValue(fmt.Sprintf("items[%d].unit_price", i))),
			TotalPrice:    parseFloat(r.FormValue(fmt.Sprintf("items[%d].total_price", i))),
			VatAmount:     parseFloat(r.FormValue(fmt.Sprintf("items[%d].vat_amount", i))),
			VatRate:       parseFloat(r.FormValue(fmt.Sprintf("items[%d].vat_rate", i))),
			VatClassifier: &vatClassifier,
		})
	}

	if err := s.svc.UpdateInvoice(r.Context(), inv, items); err != nil {
		s.setFlash(w, err.Error(), "error")
	} else {
		s.setFlash(w, "Invoice updated successfully", "success")
	}

	http.Redirect(w, r, "/invoices/"+id, http.StatusSeeOther)
}

func (s *Server) handleConfirm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	inv, err := s.svc.GetInvoiceForOrg(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if inv.Status == "pending" || inv.Status == "processing" {
		s.setFlash(w, "Invoice is currently being processed and cannot be confirmed", "error")
		http.Redirect(w, r, "/invoices/"+id, http.StatusSeeOther)
		return
	}

	if inv.Status == "duplicate" {
		s.setFlash(w, "Duplicate invoices cannot be confirmed", "error")
		http.Redirect(w, r, "/invoices/"+id, http.StatusSeeOther)
		return
	}

	createdAt := inv.CreatedAt

	if err := s.svc.ConfirmInvoice(r.Context(), id); err != nil {
		s.setFlash(w, err.Error(), "error")
		http.Redirect(w, r, "/invoices/"+id, http.StatusSeeOther)
		return
	}
	inv.Status = "ready_for_export"
	if user, ok := userFromContext(r.Context()); ok {
		baseURL := requestBaseURL(r)
		_ = s.svc.Webhook.SendInvoiceEvent(r.Context(), user.ID, "invoice.confirmed", inv, baseURL)
	}

	nextID, _ := s.svc.NextUnconfirmedInvoiceID(r.Context(), createdAt, id)
	if nextID != "" {
		s.setFlash(w, "Invoice confirmed successfully", "success")
		http.Redirect(w, r, "/invoices/"+nextID, http.StatusSeeOther)
		return
	}

	prevID, _ := s.svc.PreviousUnconfirmedInvoiceID(r.Context(), createdAt, id)
	if prevID != "" {
		s.setFlash(w, "Invoice confirmed successfully", "success")
		http.Redirect(w, r, "/invoices/"+prevID, http.StatusSeeOther)
		return
	}

	s.setFlash(w, "Invoice confirmed. All invoices have been reviewed!", "success")
	http.Redirect(w, r, "/invoices?tab=ready", http.StatusSeeOther)
}

func (s *Server) handleReprocess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	inv, err := s.svc.GetInvoiceForOrg(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if inv.Status == "pending" || inv.Status == "processing" {
		s.setFlash(w, "Invoice is already being processed", "info")
		http.Redirect(w, r, "/invoices/"+id, http.StatusSeeOther)
		return
	}

	// Set status back to pending to trigger reprocessing
	if err := s.svc.ScheduleReprocess(r.Context(), id); err != nil {
		s.setFlash(w, err.Error(), "error")
	} else {
		s.setFlash(w, "Invoice scheduled for reprocessing", "success")
	}
	http.Redirect(w, r, "/invoices/"+id, http.StatusSeeOther)
}
