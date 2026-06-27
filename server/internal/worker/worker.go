package worker

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
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
	rows, err := w.store.DB().QueryContext(ctx, "SELECT id, country, code, tariff, description, example, active, reverse_charge FROM vat_classifiers WHERE org_id = ?", orgID)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var vc domain.VatClassifier
			if err := rows.Scan(&vc.ID, &vc.Country, &vc.Code, &vc.Tariff, &vc.Description, &vc.Example, &vc.Active, &vc.ReverseCharge); err == nil {
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

	// 5. Save results
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
			buyer_individual = ?
		WHERE id = ?`,
		"processed",
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
		result.AmountWithoutVat,
		result.VatAmount,
		result.AmountWithVat,
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
		id,
	)
	if err != nil {
		return err
	}

	for _, item := range result.Items {
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
				vat_amount, vat_rate, vat_classifier, created_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.New().String(),
			id,
			item.Name,
			item.Quantity,
			unitPrice,
			item.AmountWithoutVat,
			item.VatAmount,
			vatRate,
			item.VatClassifier,
			time.Now().Unix(),
		)
		if err != nil {
			return err
		}
	}

	// 6. Upsert companies
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
		}); err != nil {
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
		}); err != nil {
			log.Printf("upsert buyer company for invoice %s: %v", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true

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
