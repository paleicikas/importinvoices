package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/paleicikas/importinvoices/server/internal/domain"
)

var ErrCompanyHasInvoices = errors.New("company has linked invoices")

type CompanyListParams struct {
	Search        string
	ColumnFilters map[int][]string
	SortCol       int
	SortDir       string
}

func (p CompanyListParams) EffectiveColumnFilters() map[int][]string {
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

func (s *Service) ListCompanies(ctx context.Context, orgID string, params CompanyListParams) ([]domain.Company, error) {
	columnMap := map[int]string{
		0: "c.title",
		1: "c.code",
		2: "c.vat_code",
		3: "c.city",
		4: "c.country",
		5: "purchases_count",
		6: "sales_count",
	}

	orderBy, ok := columnMap[params.SortCol]
	if !ok {
		orderBy = "c.title"
	}

	sortDir := "ASC"
	if strings.ToUpper(params.SortDir) == "DESC" {
		sortDir = "DESC"
	}

	query := fmt.Sprintf(`
		SELECT 
			c.id, c.org_id, c.title, c.code, c.vat_code, c.street, c.city, c.country, 
			c.postal_code, c.email, c.phone_number, c.website, c.individual, c.banks, 
			c.created_at, c.updated_at,
			(SELECT COUNT(*) FROM invoices i WHERE i.org_id = c.org_id AND i.status NOT IN ('duplicate', 'failed') AND (i.seller_code = c.code OR i.seller_vat = c.vat_code OR i.seller_name = c.title)) as purchases_count,
			(SELECT COUNT(*) FROM invoices i WHERE i.org_id = c.org_id AND i.status NOT IN ('duplicate', 'failed') AND (i.buyer_code = c.code OR i.buyer_vat = c.vat_code OR i.buyer_name = c.title)) as sales_count
		FROM companies c WHERE c.org_id = ?`)

	var args []any
	args = append(args, orgID)

	if params.Search != "" {
		query += " AND (c.title LIKE ? OR c.code LIKE ? OR c.vat_code LIKE ? OR c.city LIKE ?)"
		search := "%" + params.Search + "%"
		args = append(args, search, search, search, search)
	}

	filters := params.EffectiveColumnFilters()
	for col, vals := range filters {
		sqlCol, ok := columnMap[col]
		if !ok {
			continue
		}

		var clauses []string
		for _, val := range vals {
			clauses = append(clauses, fmt.Sprintf("LOWER(COALESCE(%s,'')) LIKE LOWER(?)", sqlCol))
			args = append(args, "%"+val+"%")
		}
		if len(clauses) > 0 {
			query += " AND (" + strings.Join(clauses, " OR ") + ")"
		}
	}

	query += fmt.Sprintf(" ORDER BY %s %s", orderBy, sortDir)

	rows, err := s.store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var companies []domain.Company
	for rows.Next() {
		var c domain.Company
		var createdAt, updatedAt int64
		if err := rows.Scan(
			&c.ID, &c.OrgID, &c.Title, &c.Code, &c.VATCode, &c.Street, &c.City, &c.Country,
			&c.PostalCode, &c.Email, &c.PhoneNumber, &c.Website, &c.Individual, &c.Banks,
			&createdAt, &updatedAt, &c.PurchasesCount, &c.SalesCount,
		); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(createdAt, 0)
		c.UpdatedAt = time.Unix(updatedAt, 0)
		companies = append(companies, c)
	}
	return companies, nil
}

func (s *Service) GetCompany(ctx context.Context, id string) (*domain.Company, error) {
	var c domain.Company
	var createdAt, updatedAt int64
	err := s.store.DB().QueryRowContext(ctx, `
		SELECT 
			c.id, c.org_id, c.title, c.code, c.vat_code, c.street, c.city, c.country, 
			c.postal_code, c.email, c.phone_number, c.website, c.individual, c.banks, 
			c.created_at, c.updated_at,
			(SELECT COUNT(*) FROM invoices i WHERE i.org_id = c.org_id AND i.status NOT IN ('duplicate', 'failed') AND (i.seller_code = c.code OR i.seller_vat = c.vat_code OR i.seller_name = c.title)) as purchases_count,
			(SELECT COUNT(*) FROM invoices i WHERE i.org_id = c.org_id AND i.status NOT IN ('duplicate', 'failed') AND (i.buyer_code = c.code OR i.buyer_vat = c.vat_code OR i.buyer_name = c.title)) as sales_count
		FROM companies c WHERE c.id = ?`, id).Scan(
		&c.ID, &c.OrgID, &c.Title, &c.Code, &c.VATCode, &c.Street, &c.City, &c.Country,
		&c.PostalCode, &c.Email, &c.PhoneNumber, &c.Website, &c.Individual, &c.Banks,
		&createdAt, &updatedAt, &c.PurchasesCount, &c.SalesCount,
	)
	if err != nil {
		return nil, err
	}
	c.CreatedAt = time.Unix(createdAt, 0)
	c.UpdatedAt = time.Unix(updatedAt, 0)
	return &c, nil
}

func (s *Service) UpsertCompany(ctx context.Context, c domain.Company) error {
	if c.Title == "" {
		return nil
	}

	// Try to find existing company
	var id string
	var err error = sql.ErrNoRows
	if c.VATCode != nil && *c.VATCode != "" {
		err = s.store.DB().QueryRowContext(ctx, "SELECT id FROM companies WHERE org_id = ? AND vat_code = ?", c.OrgID, *c.VATCode).Scan(&id)
	} else if c.Code != nil && *c.Code != "" {
		if c.Country != nil && *c.Country != "" {
			err = s.store.DB().QueryRowContext(ctx, "SELECT id FROM companies WHERE org_id = ? AND code = ? AND country = ?", c.OrgID, *c.Code, *c.Country).Scan(&id)
		} else {
			err = s.store.DB().QueryRowContext(ctx, "SELECT id FROM companies WHERE org_id = ? AND code = ?", c.OrgID, *c.Code).Scan(&id)
		}
	}

	if err != nil && c.Title != "" {
		// Fallback: match by Title + Country
		if c.Country != nil && *c.Country != "" {
			err = s.store.DB().QueryRowContext(ctx, "SELECT id FROM companies WHERE org_id = ? AND title = ? AND country = ?", c.OrgID, c.Title, *c.Country).Scan(&id)
		} else {
			err = s.store.DB().QueryRowContext(ctx, "SELECT id FROM companies WHERE org_id = ? AND title = ?", c.OrgID, c.Title).Scan(&id)
		}
	}

	now := time.Now().Unix()
	if err == nil {
		// Update existing - only update fields that are not empty in c
		_, err = s.store.DB().ExecContext(ctx, `
			UPDATE companies SET 
				title = COALESCE(NULLIF(?,''), title),
				code = COALESCE(NULLIF(?,''), code),
				vat_code = COALESCE(NULLIF(?,''), vat_code),
				street = COALESCE(NULLIF(?,''), street),
				city = COALESCE(NULLIF(?,''), city),
				country = COALESCE(NULLIF(?,''), country),
				postal_code = COALESCE(NULLIF(?,''), postal_code),
				email = COALESCE(NULLIF(?,''), email),
				phone_number = COALESCE(NULLIF(?,''), phone_number),
				website = COALESCE(NULLIF(?,''), website),
				individual = COALESCE(?, individual),
				banks = COALESCE(NULLIF(?,''), banks),
				updated_at = ?
			WHERE id = ?`,
			c.Title, c.Code, c.VATCode, c.Street, c.City, c.Country,
			c.PostalCode, c.Email, c.PhoneNumber, c.Website,
			c.Individual, c.Banks, now, id)
		return err
	}

	// Create new
	_, err = s.store.DB().ExecContext(ctx, `
		INSERT INTO companies (
			id, org_id, title, code, vat_code, street, city, country, 
			postal_code, email, phone_number, website, individual, banks, 
			created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), c.OrgID, c.Title, c.Code, c.VATCode, c.Street, c.City, c.Country,
		c.PostalCode, c.Email, c.PhoneNumber, c.Website, c.Individual, c.Banks,
		now, now)
	return err
}

func (s *Service) DeleteCompany(ctx context.Context, orgID, id string) error {
	company, err := s.GetCompany(ctx, id)
	if err != nil {
		return err
	}
	if company.OrgID != orgID {
		return sql.ErrNoRows
	}
	if company.PurchasesCount > 0 || company.SalesCount > 0 {
		return ErrCompanyHasInvoices
	}

	res, err := s.store.DB().ExecContext(ctx, "DELETE FROM companies WHERE id = ? AND org_id = ?", id, orgID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}