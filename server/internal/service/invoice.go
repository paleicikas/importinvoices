package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/reqctx"
)

type InvoiceListParams struct {
	Tab           string
	Search        string
	ColumnFilters map[int][]string
	SortCol       int
	SortDir       string
	Limit         int
	Offset        int
}

func (p InvoiceListParams) EffectiveColumnFilters() map[int][]string {
	out := make(map[int][]string)
	for col, vals := range p.ColumnFilters {
		var active []string
		for _, v := range vals {
			v = strings.TrimSpace(v)
			if v != "" {
				active = append(active, v)
			}
		}
		if len(active) > 0 {
			out[col] = active
		}
	}
	return out
}

type InvoiceCounts struct {
	Processing int
	Ready      int
	Export     int
	Exported   int
	Failed     int
	Duplicates int
}

func (s *Service) CreateInvoice(ctx context.Context, inv *domain.Invoice) error {
	if inv.ID == "" {
		inv.ID = time.Now().Format("20060102150405") + "-" + inv.Checksum
	}
	_, err := s.store.DB().ExecContext(ctx, `
		INSERT INTO invoices (id, user_id, org_id, status, filename, checksum, storage_path, duplicate_of_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inv.ID, inv.UserID, inv.OrgID, inv.Status, inv.Filename, inv.Checksum, inv.StoragePath, inv.DuplicateOfID, time.Now().Unix(), time.Now().Unix())
	return err
}

func (s *Service) organizationID(ctx context.Context) (string, error) {
	org, ok := reqctx.Organization(ctx)
	if ok && org != nil {
		return org.ID, nil
	}
	org, err := s.GetOrganization(ctx)
	if err != nil {
		return "", err
	}
	return org.ID, nil
}

func (s *Service) ListInvoices(ctx context.Context, params InvoiceListParams) ([]domain.Invoice, int, error) {
	orgID, err := s.organizationID(ctx)
	if err != nil {
		return nil, 0, err
	}

	baseQuery := "FROM invoices"
	var args []any

	whereClauses := []string{"org_id = ?"}
	args = append(args, orgID)
	
	switch params.Tab {
	case "processing":
		whereClauses = append(whereClauses, "status IN ('pending', 'processing')")
	case "ready":
		whereClauses = append(whereClauses, "status = 'processed'")
	case "export":
		whereClauses = append(whereClauses, "status = 'ready_for_export'")
	case "exported":
		whereClauses = append(whereClauses, "status = 'exported'")
	case "failed":
		whereClauses = append(whereClauses, "status = 'failed'")
	case "duplicates":
		whereClauses = append(whereClauses, "status = 'duplicate'")
	}

	if params.Search != "" {
		whereClauses = append(whereClauses, "(filename LIKE ? OR seller_name LIKE ? OR series_and_number LIKE ? OR buyer_name LIKE ?)")
		search := "%" + params.Search + "%"
		args = append(args, search, search, search, search)
	}

	// Apply column filters
	columnMap := map[int]string{
		0:  "created_at",
		1:  "series_and_number",
		2:  "type",
		3:  "issue_date",
		4:  "supply_date",
		5:  "payment_due_date",
		6:  "seller_name",
		7:  "seller_code",
		8:  "seller_vat",
		9:  "buyer_name",
		10: "buyer_code",
		11: "buyer_vat",
		12: "amount_without_vat",
		13: "vat_amount",
		14: "amount_with_vat",
		15: "currency",
		16: "status",
		35: "vat_classifier",
	}

	filters := params.EffectiveColumnFilters()
	for col, vals := range filters {
		sqlCol, ok := columnMap[col]
		if !ok && col != 100 && col != 101 && col != 35 {
			continue
		}

		if col == 35 {
			var subClauses []string
			for _, val := range vals {
				subClauses = append(subClauses, "id IN (SELECT invoice_id FROM invoice_items WHERE vat_classifier = ?)")
				args = append(args, val)
			}
			if len(subClauses) > 0 {
				whereClauses = append(whereClauses, "("+strings.Join(subClauses, " OR ")+")")
			}
			continue
		}

		if col == 0 || col == 3 || col == 4 || col == 5 {
			from := ""
			to := ""
			if len(vals) >= 1 { from = vals[0] }
			if len(vals) >= 2 { to = vals[1] }

			if from != "" && to != "" {
				f, _ := time.Parse("2006-01-02", from)
				t, _ := time.Parse("2006-01-02", to)
				whereClauses = append(whereClauses, fmt.Sprintf("%s BETWEEN ? AND ?", sqlCol))
				args = append(args, f.Unix(), t.Add(24*time.Hour).Unix())
			} else if from != "" {
				f, _ := time.Parse("2006-01-02", from)
				whereClauses = append(whereClauses, fmt.Sprintf("%s >= ?", sqlCol))
				args = append(args, f.Unix())
			} else if to != "" {
				t, _ := time.Parse("2006-01-02", to)
				whereClauses = append(whereClauses, fmt.Sprintf("%s <= ?", sqlCol))
				args = append(args, t.Add(24*time.Hour).Unix())
			}
			continue
		}

		if col == 100 || col == 101 {
			var subClauses []string
			cols := []string{"seller_name", "seller_code", "seller_vat"}
			if col == 101 { cols = []string{"buyer_name", "buyer_code", "buyer_vat"} }
			for _, val := range vals {
				for _, c := range cols {
					subClauses = append(subClauses, fmt.Sprintf("LOWER(COALESCE(%s,'')) LIKE LOWER(?)", c))
					args = append(args, "%"+val+"%")
				}
			}
			if len(subClauses) > 0 {
				whereClauses = append(whereClauses, "("+strings.Join(subClauses, " OR ")+")")
			}
			continue
		}

		var clauses []string
		for _, val := range vals {
			// Use exact match for specific columns, LIKE for others
			useExact := false
			switch sqlCol {
			case "status", "type", "buyer_code", "seller_code", "currency", "series_and_number", "buyer_name", "seller_name":
				useExact = true
			}

			if useExact {
				clauses = append(clauses, fmt.Sprintf("%s = ?", sqlCol))
				args = append(args, val)
			} else {
				clauses = append(clauses, fmt.Sprintf("LOWER(COALESCE(%s,'')) LIKE LOWER(?)", sqlCol))
				args = append(args, "%"+val+"%")
			}
		}
		if len(clauses) > 0 {
			whereClauses = append(whereClauses, "("+strings.Join(clauses, " OR ")+")")
		}
	}

	where := ""
	if len(whereClauses) > 0 {
		where = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// 1. Get total count
	var total int
	countQuery := "SELECT COUNT(*) " + baseQuery + where
	err = s.store.DB().QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// 2. Get data
	query := `
		SELECT 
			id, user_id, org_id, status, filename, checksum, storage_path, preview_path, created_at, updated_at,
			type, series_and_number, currency, issue_date, supply_date, payment_due_date,
			seller_name, seller_code, seller_vat, buyer_name, buyer_code, buyer_vat,
			amount_without_vat, vat_amount, amount_with_vat, seller_banks, buyer_banks,
			ocr_text, is_invoice, original_invoice_public_id,
			seller_street, seller_city, seller_country, seller_postal_code, seller_email, seller_phone_number, seller_website, seller_individual,
			buyer_street, buyer_city, buyer_country, buyer_postal_code, buyer_email, buyer_phone_number, buyer_website, buyer_individual,
			duplicate_of_id, error_message,
			(SELECT GROUP_CONCAT(DISTINCT vat_classifier) FROM invoice_items WHERE invoice_id = invoices.id) as vat_codes ` +
		baseQuery + where

	sortColName, ok := columnMap[params.SortCol]
	if !ok { sortColName = "created_at" }
	sortDir := "DESC"
	if strings.ToUpper(params.SortDir) == "ASC" { sortDir = "ASC" }
	query += fmt.Sprintf(" ORDER BY %s %s", sortColName, sortDir)

	if params.Limit > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, params.Limit, params.Offset)
	}

	rows, err := s.store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var invoices []domain.Invoice
	for rows.Next() {
		var inv domain.Invoice
		var createdAt, updatedAt int64
		var issueDate, supplyDate, paymentDueDate *int64
		if err := rows.Scan(
			&inv.ID, &inv.UserID, &inv.OrgID, &inv.Status, &inv.Filename, &inv.Checksum, &inv.StoragePath, &inv.PreviewPath, &createdAt, &updatedAt,
			&inv.Type, &inv.SeriesAndNumber, &inv.Currency, &issueDate, &supplyDate, &paymentDueDate,
			&inv.SellerName, &inv.SellerCode, &inv.SellerVAT, &inv.BuyerName, &inv.BuyerCode, &inv.BuyerVAT,
			&inv.AmountWithoutVat, &inv.VatAmount, &inv.AmountWithVat, &inv.SellerBanks, &inv.BuyerBanks,
			&inv.OcrText, &inv.IsInvoice, &inv.OriginalInvoicePublicID,
			&inv.SellerStreet, &inv.SellerCity, &inv.SellerCountry, &inv.SellerPostalCode, &inv.SellerEmail, &inv.SellerPhoneNumber, &inv.SellerWebsite, &inv.SellerIndividual,
			&inv.BuyerStreet, &inv.BuyerCity, &inv.BuyerCountry, &inv.BuyerPostalCode, &inv.BuyerEmail, &inv.BuyerPhoneNumber, &inv.BuyerWebsite, &inv.BuyerIndividual,
			&inv.DuplicateOfID, &inv.ErrorMessage, &inv.VatCodes,
		); err != nil {
			return nil, 0, err
		}
		inv.CreatedAt = time.Unix(createdAt, 0)
		inv.UpdatedAt = time.Unix(updatedAt, 0)
		if issueDate != nil { t := time.Unix(*issueDate, 0); inv.IssueDate = &t }
		if supplyDate != nil { t := time.Unix(*supplyDate, 0); inv.SupplyDate = &t }
		if paymentDueDate != nil { t := time.Unix(*paymentDueDate, 0); inv.PaymentDueDate = &t }
		invoices = append(invoices, inv)
	}
	return invoices, total, nil
}

func (s *Service) ConfirmInvoice(ctx context.Context, id string) error {
	orgID, err := s.organizationID(ctx)
	if err != nil {
		return err
	}
	_, err = s.store.DB().ExecContext(ctx,
		"UPDATE invoices SET status = ?, updated_at = ? WHERE id = ? AND org_id = ?",
		"ready_for_export", time.Now().Unix(), id, orgID)
	return err
}

func (s *Service) ScheduleReprocess(ctx context.Context, id string) error {
	orgID, err := s.organizationID(ctx)
	if err != nil {
		return err
	}
	_, err = s.store.DB().ExecContext(ctx,
		"UPDATE invoices SET status = ?, updated_at = ? WHERE id = ? AND org_id = ?",
		"pending", time.Now().Unix(), id, orgID)
	if err != nil {
		return err
	}
	if s.worker != nil {
		s.worker.Queue(id)
	}
	return nil
}

func (s *Service) GetInvoice(ctx context.Context, id string) (*domain.Invoice, []domain.InvoiceItem, error) {
	var inv domain.Invoice
	var createdAt, updatedAt int64
	var issueDate, supplyDate, paymentDueDate *int64
	err := s.store.DB().QueryRowContext(ctx, `
		SELECT 
			id, user_id, org_id, status, filename, checksum, storage_path, preview_path, created_at, updated_at,
			type, series_and_number, currency, issue_date, supply_date, payment_due_date,
			seller_name, seller_code, seller_vat, buyer_name, buyer_code, buyer_vat,
			amount_without_vat, vat_amount, amount_with_vat, seller_banks, buyer_banks,
			ocr_text, is_invoice, original_invoice_public_id,
			seller_street, seller_city, seller_country, seller_postal_code, seller_email, seller_phone_number, seller_website, seller_individual,
			buyer_street, buyer_city, buyer_country, buyer_postal_code, buyer_email, buyer_phone_number, buyer_website, buyer_individual,
			duplicate_of_id, error_message,
			(SELECT GROUP_CONCAT(DISTINCT vat_classifier) FROM invoice_items WHERE invoice_id = invoices.id) as vat_codes
		FROM invoices WHERE id = ?`, id).Scan(
		&inv.ID, &inv.UserID, &inv.OrgID, &inv.Status, &inv.Filename, &inv.Checksum, &inv.StoragePath, &inv.PreviewPath, &createdAt, &updatedAt,
		&inv.Type, &inv.SeriesAndNumber, &inv.Currency, &issueDate, &supplyDate, &paymentDueDate,
		&inv.SellerName, &inv.SellerCode, &inv.SellerVAT, &inv.BuyerName, &inv.BuyerCode, &inv.BuyerVAT,
		&inv.AmountWithoutVat, &inv.VatAmount, &inv.AmountWithVat, &inv.SellerBanks, &inv.BuyerBanks,
		&inv.OcrText, &inv.IsInvoice, &inv.OriginalInvoicePublicID,
		&inv.SellerStreet, &inv.SellerCity, &inv.SellerCountry, &inv.SellerPostalCode, &inv.SellerEmail, &inv.SellerPhoneNumber, &inv.SellerWebsite, &inv.SellerIndividual,
		&inv.BuyerStreet, &inv.BuyerCity, &inv.BuyerCountry, &inv.BuyerPostalCode, &inv.BuyerEmail, &inv.BuyerPhoneNumber, &inv.BuyerWebsite, &inv.BuyerIndividual,
		&inv.DuplicateOfID, &inv.ErrorMessage, &inv.VatCodes,
	)
	if err != nil {
		return nil, nil, err
	}
	inv.CreatedAt = time.Unix(createdAt, 0)
	inv.UpdatedAt = time.Unix(updatedAt, 0)
	if issueDate != nil {
		t := time.Unix(*issueDate, 0)
		inv.IssueDate = &t
	}
	if supplyDate != nil {
		t := time.Unix(*supplyDate, 0)
		inv.SupplyDate = &t
	}
	if paymentDueDate != nil {
		t := time.Unix(*paymentDueDate, 0)
		inv.PaymentDueDate = &t
	}

	rows, err := s.store.DB().QueryContext(ctx, `
		SELECT id, invoice_id, description, quantity, unit_price, total_price, vat_amount, vat_rate, vat_classifier, created_at
		FROM invoice_items WHERE invoice_id = ? ORDER BY created_at ASC`, id)
	if err != nil {
		return &inv, nil, nil
	}
	defer func() { _ = rows.Close() }()

	var items []domain.InvoiceItem
	for rows.Next() {
		var item domain.InvoiceItem
		var createdAt int64
		if err := rows.Scan(
			&item.ID, &item.InvoiceID, &item.Description, &item.Quantity, &item.UnitPrice, &item.TotalPrice, &item.VatAmount, &item.VatRate, &item.VatClassifier, &createdAt,
		); err != nil {
			return nil, nil, err
		}
		item.CreatedAt = time.Unix(createdAt, 0)
		items = append(items, item)
	}

	return &inv, items, nil
}

func (s *Service) GetInvoiceForOrg(ctx context.Context, id string) (*domain.Invoice, error) {
	inv, _, err := s.GetInvoice(ctx, id)
	if err != nil {
		return nil, err
	}

	org, ok := reqctx.Organization(ctx)
	if !ok || org == nil {
		var orgErr error
		org, orgErr = s.GetOrganization(ctx)
		if orgErr != nil {
			return nil, orgErr
		}
	}
	if inv.OrgID != org.ID {
		return nil, sql.ErrNoRows
	}
	return inv, nil
}

func (s *Service) reviewQueueWhere(ctx context.Context) (string, []any) {
	org, _ := reqctx.Organization(ctx)
	orgID := ""
	if org != nil {
		orgID = org.ID
	}
	return "status = 'processed' AND org_id = ?", []any{orgID}
}

func (s *Service) CountUnconfirmedInvoices(ctx context.Context) (int, error) {
	where, args := s.reviewQueueWhere(ctx)
	var count int
	err := s.store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM invoices WHERE "+where, args...).Scan(&count)
	return count, err
}

func (s *Service) CountUnconfirmedBefore(ctx context.Context, createdAt time.Time, currentID string) (int, error) {
	where, args := s.reviewQueueWhere(ctx)
	query := "SELECT COUNT(*) FROM invoices WHERE " + where + " AND (created_at > ? OR (created_at = ? AND id > ?))"
	args = append(args, createdAt.Unix(), createdAt.Unix(), currentID)
	var count int
	err := s.store.DB().QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (s *Service) NextUnconfirmedInvoiceID(ctx context.Context, createdAt time.Time, currentID string) (string, error) {
	where, args := s.reviewQueueWhere(ctx)
	query := "SELECT id FROM invoices WHERE " + where + " AND (created_at < ? OR (created_at = ? AND id < ?)) ORDER BY created_at DESC, id DESC LIMIT 1"
	args = append(args, createdAt.Unix(), createdAt.Unix(), currentID)
	var id string
	err := s.store.DB().QueryRowContext(ctx, query, args...).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

func (s *Service) PreviousUnconfirmedInvoiceID(ctx context.Context, createdAt time.Time, currentID string) (string, error) {
	where, args := s.reviewQueueWhere(ctx)
	query := "SELECT id FROM invoices WHERE " + where + " AND (created_at > ? OR (created_at = ? AND id > ?)) ORDER BY created_at ASC, id ASC LIMIT 1"
	args = append(args, createdAt.Unix(), createdAt.Unix(), currentID)
	var id string
	err := s.store.DB().QueryRowContext(ctx, query, args...).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

func (s *Service) GetFirstUnconfirmedInvoiceID(ctx context.Context) (string, error) {
	where, args := s.reviewQueueWhere(ctx)
	query := "SELECT id FROM invoices WHERE " + where + " ORDER BY created_at DESC, id DESC LIMIT 1"
	var id string
	err := s.store.DB().QueryRowContext(ctx, query, args...).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

func (s *Service) GetInvoiceReviewQueue(ctx context.Context, currentID string, createdAt time.Time) (string, string, int, int, error) {
	nextID, err := s.NextUnconfirmedInvoiceID(ctx, createdAt, currentID)
	if err != nil {
		return "", "", 0, 0, err
	}
	prevID, err := s.PreviousUnconfirmedInvoiceID(ctx, createdAt, currentID)
	if err != nil {
		return "", "", 0, 0, err
	}
	countBefore, err := s.CountUnconfirmedBefore(ctx, createdAt, currentID)
	if err != nil {
		return "", "", 0, 0, err
	}
	totalCount, err := s.CountUnconfirmedInvoices(ctx)
	if err != nil {
		return "", "", 0, 0, err
	}
	return nextID, prevID, countBefore + 1, totalCount, nil
}

func (s *Service) UpdateInvoice(ctx context.Context, inv *domain.Invoice, items []domain.InvoiceItem) error {
	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	toUnix := func(t *time.Time) *int64 {
		if t == nil {
			return nil
		}
		u := t.Unix()
		return &u
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE invoices SET 
			series_and_number = ?, issue_date = ?, supply_date = ?, payment_due_date = ?,
			currency = ?, amount_without_vat = ?, vat_amount = ?, amount_with_vat = ?,
			seller_name = ?, seller_code = ?, seller_vat = ?, seller_street = ?, seller_city = ?, seller_country = ?,
			buyer_name = ?, buyer_code = ?, buyer_vat = ?, buyer_street = ?, buyer_city = ?, buyer_country = ?,
			updated_at = ?
		WHERE id = ?`,
		inv.SeriesAndNumber, toUnix(inv.IssueDate), toUnix(inv.SupplyDate), toUnix(inv.PaymentDueDate),
		inv.Currency, inv.AmountWithoutVat, inv.VatAmount, inv.AmountWithVat,
		inv.SellerName, inv.SellerCode, inv.SellerVAT, inv.SellerStreet, inv.SellerCity, inv.SellerCountry,
		inv.BuyerName, inv.BuyerCode, inv.BuyerVAT, inv.BuyerStreet, inv.BuyerCity, inv.BuyerCountry,
		time.Now().Unix(), inv.ID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM invoice_items WHERE invoice_id = ?", inv.ID)
	if err != nil {
		return err
	}

	for _, item := range items {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO invoice_items (id, invoice_id, description, quantity, unit_price, total_price, vat_amount, vat_rate, vat_classifier, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, inv.ID, item.Description, item.Quantity, item.UnitPrice, item.TotalPrice, item.VatAmount, item.VatRate, item.VatClassifier, time.Now().Unix())
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Service) GetInvoiceCounts(ctx context.Context) (InvoiceCounts, error) {
	org, ok := reqctx.Organization(ctx)
	if !ok || org == nil {
		var err error
		org, err = s.GetOrganization(ctx)
		if err != nil {
			return InvoiceCounts{}, err
		}
	}
	return s.CountInvoices(ctx, org.ID)
}

func (s *Service) CountInvoices(ctx context.Context, orgID string) (InvoiceCounts, error) {
	var counts InvoiceCounts
	err := s.store.DB().QueryRowContext(ctx, `
		SELECT 
			COUNT(CASE WHEN status IN ('pending', 'processing') THEN 1 END),
			COUNT(CASE WHEN status = 'processed' THEN 1 END),
			COUNT(CASE WHEN status = 'ready_for_export' THEN 1 END),
			COUNT(CASE WHEN status = 'exported' THEN 1 END),
			COUNT(CASE WHEN status = 'failed' THEN 1 END),
			COUNT(CASE WHEN status = 'duplicate' THEN 1 END)
		FROM invoices WHERE org_id = ?`, orgID).Scan(
		&counts.Processing, &counts.Ready, &counts.Export, &counts.Exported, &counts.Failed, &counts.Duplicates,
	)
	return counts, err
}

func (s *Service) ListInvoicesByCompany(ctx context.Context, company *domain.Company, asBuyer bool, params InvoiceListParams) ([]domain.Invoice, int, error) {
	if params.ColumnFilters == nil {
		params.ColumnFilters = make(map[int][]string)
	}
	
	col := 100 // Seller
	if asBuyer {
		col = 101 // Buyer
	}
	
	params.ColumnFilters[col] = append(params.ColumnFilters[col], company.Title)
	if company.Code != nil && *company.Code != "" {
		params.ColumnFilters[col] = append(params.ColumnFilters[col], *company.Code)
	}
	if company.VATCode != nil && *company.VATCode != "" {
		params.ColumnFilters[col] = append(params.ColumnFilters[col], *company.VATCode)
	}
	
	return s.ListInvoices(ctx, params)
}
