package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/media"
	"github.com/paleicikas/importinvoices/server/internal/service"
)

type Worker struct {
	store *db.Store
	svc   *service.Service
	media *media.MediaService
	queue chan string
}

func New(store *db.Store, svc *service.Service, media *media.MediaService) *Worker {
	return &Worker{
		store: store,
		svc:   svc,
		media: media,
		queue: make(chan string, 100),
	}
}

func (w *Worker) Queue(invoiceID string) {
	w.queue <- invoiceID
}

func (w *Worker) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-w.queue:
			if err := w.process(ctx, id); err != nil {
				log.Printf("Worker error processing %s: %v", id, err)
			}
		}
	}
}

func (w *Worker) setInvoiceStatus(ctx context.Context, id, status string, errMsg *string) error {
	_, err := w.store.DB().ExecContext(ctx,
		"UPDATE invoices SET status = ?, error_message = ?, updated_at = ? WHERE id = ?",
		status, errMsg, time.Now().Unix(), id)
	return err
}

func (w *Worker) process(ctx context.Context, id string) (err error) {
	// 1. Get invoice
	var relPath string
	var userID string
	var orgID string
	err = w.store.DB().QueryRowContext(ctx, "SELECT user_id, org_id, storage_path FROM invoices WHERE id = ?", id).Scan(&userID, &orgID, &relPath)
	if err != nil {
		return err
	}
	invPath := filepath.Join(w.svc.Storage().BasePath(), relPath)

	// 2. Set status to processing
	if err = w.setInvoiceStatus(ctx, id, "processing", nil); err != nil {
		return err
	}

	committed := false
	defer func() {
		if err != nil && !committed {
			errMsg := err.Error()
			if execErr := w.setInvoiceStatus(ctx, id, "failed", &errMsg); execErr != nil {
				log.Printf("failed to mark invoice %s as failed: %v", id, execErr)
			}
		}
	}()

	// 3. Get processor
	proc, err := w.svc.GetProcessor(ctx)
	if err != nil {
		return err
	}

	// 3.5 Get VAT Classifiers
	var vatClassifiers []domain.VatClassifier
	rows, err := w.store.DB().QueryContext(ctx, `
		SELECT 
			id, country, code, tariff, description, example, 
			receiving_rule, issued_rule, active, reverse_charge, 
			purchase_account, include_in_isaf 
		FROM vat_classifiers 
		WHERE org_id = ? AND active = 1`, orgID)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var vc domain.VatClassifier
			if err := rows.Scan(
				&vc.ID, &vc.Country, &vc.Code, &vc.Tariff, &vc.Description, &vc.Example,
				&vc.ReceivingRule, &vc.IssuedRule, &vc.Active, &vc.ReverseCharge,
				&vc.PurchaseAccount, &vc.IncludeInIsaf,
			); err == nil {
				vatClassifiers = append(vatClassifiers, vc)
			}
		}
	}

	// 4. Process
	imagePaths, err := w.media.ConvertToImages(ctx, invPath)
	if err != nil {
		log.Printf("Failed to convert %s to images: %v", invPath, err)
		// Fallback to original path if conversion fails
		imagePaths = []string{invPath}
	} else {
		defer func() {
			for _, p := range imagePaths {
				if p != invPath {
					_ = os.Remove(p)
				}
			}
		}()
	}

	result, err := proc.Process(ctx, imagePaths, vatClassifiers)
	if err != nil {
		return err
	}

	// 4.5 Business-key dedup: even if the file bytes differ, an invoice with the
	// same (org, seller_vat, series_and_number) as another non-duplicate invoice
	// is a duplicate of it. This catches re-scans of the same document that
	// checksum-only dedup would miss.
	newStatus := "processed"
	var duplicateOfID *string
	if result.SellerVatIdentificationNumber != "" && result.SeriesAndNumber != "" {
		var existingID string
		qErr := w.store.DB().QueryRowContext(ctx, `
			SELECT id FROM invoices
			WHERE org_id = ? AND seller_vat = ? AND series_and_number = ?
			  AND id != ? AND status != 'duplicate'
			LIMIT 1`, orgID, result.SellerVatIdentificationNumber, result.SeriesAndNumber, id,
		).Scan(&existingID)
		if qErr == nil {
			newStatus = "duplicate"
			duplicateOfID = &existingID
		} else if qErr != sql.ErrNoRows {
			return qErr
		}
	}

	// 4.6 Post-process VAT validation (P2-3.b) and tariff resolution (P2-3.c).
	// Each item's AI-assigned vat_classifier is checked against the org's active
	// catalog. Unknown codes produce a vat_warning on the invoice; matched codes
	// persist the classifier's tariff on the item row.
	classifierByCode := make(map[string]domain.VatClassifier, len(vatClassifiers))
	for _, vc := range vatClassifiers {
		classifierByCode[vc.Code] = vc
	}
	itemTariffs := make([]*float64, len(result.Items))
	var unknownCodes []string
	for i, item := range result.Items {
		if item.VatClassifier == "" {
			continue
		}
		vc, ok := classifierByCode[item.VatClassifier]
		if !ok {
			unknownCodes = append(unknownCodes, item.VatClassifier)
			continue
		}
		t := vc.Tariff
		itemTariffs[i] = &t
	}
	var vatWarning string
	if len(unknownCodes) > 0 {
		vatWarning = "unknown VAT classifier code(s): " + strings.Join(unknownCodes, ", ")
	}

	// 5. Save results
	var vatWarningArg interface{}
	if vatWarning != "" {
		vatWarningArg = vatWarning
	}

	tx, err := w.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, "DELETE FROM invoice_items WHERE invoice_id = ?", id); err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE invoices SET
			status = ?,
			duplicate_of_id = ?,
			error_message = NULL,
			updated_at = ?,
			type = ?,
			series_and_number = ?,
			currency = ?,
			issue_date = ?,
			supply_date = ?,
			payment_due_date = ?,
			seller_name = ?,
			seller_code = ?,
			seller_vat = ?,
			buyer_name = ?,
			buyer_code = ?,
			buyer_vat = ?,
			amount_without_vat = ?,
			vat_amount = ?,
			amount_with_vat = ?,
			seller_banks = ?,
			buyer_banks = ?,
			ocr_text = ?,
			is_invoice = ?,
			original_invoice_public_id = ?,
			seller_street = ?,
			seller_city = ?,
			seller_country = ?,
			seller_postal_code = ?,
			seller_email = ?,
			seller_phone_number = ?,
			seller_website = ?,
			seller_individual = ?,
			buyer_street = ?,
			buyer_city = ?,
			buyer_country = ?,
			buyer_postal_code = ?,
			buyer_email = ?,
			buyer_phone_number = ?,
			buyer_website = ?,
			buyer_individual = ?,
			vat_warning = ?
		WHERE id = ?`,
		newStatus,
		duplicateOfID,
		time.Now().Unix(),
		toInt(result.Type),
		result.SeriesAndNumber,
		result.Currency,
		parseDate(result.IssueDate),
		parseDate(result.SupplyDate),
		parseDate(result.PaymentDueDate),
		result.SellerCompanyName,
		result.SellerCompanyCode,
		result.SellerVatIdentificationNumber,
		result.BuyerCompanyName,
		result.BuyerCompanyCode,
		result.BuyerVatIdentificationNumber,
		toCents(result.AmountWithoutVat),
		toCents(result.VatAmount),
		toCents(result.AmountWithVat),
		jsonMarshal(result.SellerBanks),
		jsonMarshal(result.BuyerBanks),
		result.OcrText,
		result.IsInvoice,
		result.OriginalInvoicePublicId,
		result.SellerStreet,
		result.SellerCity,
		result.SellerCountry,
		result.SellerPostalCode,
		result.SellerEmail,
		result.SellerPhoneNumber,
		result.SellerWebsite,
		result.SellerIndividual,
		result.BuyerStreet,
		result.BuyerCity,
		result.BuyerCountry,
		result.BuyerPostalCode,
		result.BuyerEmail,
		result.BuyerPhoneNumber,
		result.BuyerWebsite,
		result.BuyerIndividual,
		vatWarningArg,
		id,
	)
	if err != nil {
		return err
	}

	for idx, item := range result.Items {
		// Duplicates don't need their own line items; the original holds them.
		if newStatus == "duplicate" {
			break
		}
		unitPrice := 0.0
		if item.Quantity > 0 {
			unitPrice = item.AmountWithoutVat / item.Quantity
		}
		vatRate := 0.0
		if item.AmountWithoutVat > 0 {
			vatRate = (item.VatAmount / item.AmountWithoutVat) * 100
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO invoice_items (
				id, invoice_id, description, quantity, unit_price, total_price,
				vat_amount, vat_rate, vat_classifier, tariff, created_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.New().String(),
			id,
			item.Name,
			item.Quantity,
			toCents(unitPrice),
			toCents(item.AmountWithVat),
			toCents(item.VatAmount),
			vatRate,
			item.VatClassifier,
			itemTariffs[idx],
			time.Now().Unix(),
		)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true

	// Upsert companies after the invoice transaction commits to avoid holding
	// a write lock while other requests may also need the database.
	if result.SellerCompanyName != "" {
		if err := w.svc.UpsertCompany(ctx, domain.Company{
			OrgID:       orgID,
			Title:       result.SellerCompanyName,
			Code:        &result.SellerCompanyCode,
			VATCode:     &result.SellerVatIdentificationNumber,
			Street:      &result.SellerStreet,
			City:        &result.SellerCity,
			Country:     &result.SellerCountry,
			PostalCode:  &result.SellerPostalCode,
			Email:       &result.SellerEmail,
			PhoneNumber: &result.SellerPhoneNumber,
			Website:     &result.SellerWebsite,
			Individual:  &result.SellerIndividual,
			Banks:       jsonMarshal(result.SellerBanks),
		}, nil); err != nil {
			log.Printf("upsert seller company for invoice %s: %v", id, err)
		}
	}
	if result.BuyerCompanyName != "" {
		if err := w.svc.UpsertCompany(ctx, domain.Company{
			OrgID:       orgID,
			Title:       result.BuyerCompanyName,
			Code:        &result.BuyerCompanyCode,
			VATCode:     &result.BuyerVatIdentificationNumber,
			Street:      &result.BuyerStreet,
			City:        &result.BuyerCity,
			Country:     &result.BuyerCountry,
			PostalCode:  &result.BuyerPostalCode,
			Email:       &result.BuyerEmail,
			PhoneNumber: &result.BuyerPhoneNumber,
			Website:     &result.BuyerWebsite,
			Individual:  &result.BuyerIndividual,
			Banks:       jsonMarshal(result.BuyerBanks),
		}, nil); err != nil {
			log.Printf("upsert buyer company for invoice %s: %v", id, err)
		}
	}

	// 8. Send webhook
	inv, _, _ := w.svc.GetInvoice(ctx, id)
	if inv != nil {
		_ = w.svc.Webhook.SendWebhook(ctx, userID, "invoice.processed", inv)
	}

	return nil
}

func parseDate(s string) *int64 {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	unix := t.Unix()
	return &unix
}

func jsonMarshal(v any) *string {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	s := string(data)
	if s == "null" || s == "[]" || s == "{}" {
		return nil
	}
	return &s
}

func toInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

// toCents converts a euro float to integer cents (rounded to nearest cent).
func toCents(v float64) int64 {
	return int64(math.Round(v * 100))
}
