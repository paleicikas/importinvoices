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

// normalizeVATCode strips a leading 2-letter ISO country prefix from a VAT
// identifier (e.g. "LT123456789" -> "123456789", "DE123" -> "123") so that the
// same company is matched and stored once regardless of whether the invoice
// printed the VAT number with or without the country prefix. Codes without a
// leading 2-letter prefix are returned unchanged.
func normalizeVATCode(s string) string {
	s = strings.TrimSpace(strings.ToUpper(s))
	if len(s) > 2 && isAlphaCountryPrefix(s[:2]) && !isAlpha(s[2:3]) {
		return s[2:]
	}
	return s
}

func isAlphaCountryPrefix(p string) bool {
	if len(p) != 2 {
		return false
	}
	return isAlpha(p)
}

func isAlpha(s string) bool {
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return true
}

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
			(SELECT COUNT(*) FROM invoices i WHERE i.org_id = c.org_id AND i.status NOT IN ('duplicate', 'failed') AND i.seller_company_id = c.id) as purchases_count,
			(SELECT COUNT(*) FROM invoices i WHERE i.org_id = c.org_id AND i.status NOT IN ('duplicate', 'failed') AND i.buyer_company_id = c.id) as sales_count
		FROM companies c WHERE c.org_id = ?`)

	var args []any
	args = append(args, orgID)

	if params.Search != "" {
		query += " AND (c.title LIKE ? ESCAPE '\\' OR c.code LIKE ? ESCAPE '\\' OR c.vat_code LIKE ? ESCAPE '\\' OR c.city LIKE ? ESCAPE '\\')"
		search := "%" + escapeLike(params.Search) + "%"
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
			(SELECT COUNT(*) FROM invoices i WHERE i.org_id = c.org_id AND i.status NOT IN ('duplicate', 'failed') AND i.seller_company_id = c.id) as purchases_count,
			(SELECT COUNT(*) FROM invoices i WHERE i.org_id = c.org_id AND i.status NOT IN ('duplicate', 'failed') AND i.buyer_company_id = c.id) as sales_count
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

// GetCompanyForOrg returns the company only if it belongs to the organization
// resolved from the context. Cross-org reads return sql.ErrNoRows without
// revealing the company exists.
func (s *Service) GetCompanyForOrg(ctx context.Context, id string) (*domain.Company, error) {
	orgID, err := s.organizationID(ctx)
	if err != nil {
		return nil, err
	}
	var c domain.Company
	var createdAt, updatedAt int64
	err = s.store.DB().QueryRowContext(ctx, `
		SELECT
			c.id, c.org_id, c.title, c.code, c.vat_code, c.street, c.city, c.country,
			c.postal_code, c.email, c.phone_number, c.website, c.individual, c.banks,
			c.created_at, c.updated_at,
			(SELECT COUNT(*) FROM invoices i WHERE i.org_id = c.org_id AND i.status NOT IN ('duplicate', 'failed') AND i.seller_company_id = c.id) as purchases_count,
			(SELECT COUNT(*) FROM invoices i WHERE i.org_id = c.org_id AND i.status NOT IN ('duplicate', 'failed') AND i.buyer_company_id = c.id) as sales_count
		FROM companies c WHERE c.id = ? AND c.org_id = ?`, id, orgID).Scan(
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

func (s *Service) UpsertCompany(ctx context.Context, c domain.Company, tx *sql.Tx) (string, error) {
	if c.Title == "" {
		return "", nil
	}

	// P2-4.a: normalize the VAT code (strip a leading 2-letter country prefix
	// such as "LT123456789" -> "123456789") so company matching and the
	// (org_id, vat_code) uniqueness index are not fooled by prefix variants.
	if c.VATCode != nil {
		n := normalizeVATCode(*c.VATCode)
		c.VATCode = &n
	}

	db := s.store.DB()

	id, err := s.matchCompanyID(ctx, c, tx)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}

	now := time.Now().Unix()
	if err == nil {
		// Update existing - only update fields that are not empty in c
		query := `
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
			WHERE id = ?`
		args := []any{
			c.Title, c.Code, c.VATCode, c.Street, c.City, c.Country,
			c.PostalCode, c.Email, c.PhoneNumber, c.Website,
			c.Individual, c.Banks, now, id,
		}
		if tx != nil {
			_, err = tx.ExecContext(ctx, query, args...)
		} else {
			_, err = db.ExecContext(ctx, query, args...)
		}
		if err != nil {
			return "", err
		}
		return id, nil
	}

	// Create new
	newID := uuid.New().String()
	query := `
		INSERT INTO companies (
			id, org_id, title, code, vat_code, street, city, country,
			postal_code, email, phone_number, website, individual, banks,
			created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	args := []any{
		newID, c.OrgID, c.Title, c.Code, c.VATCode, c.Street, c.City, c.Country,
		c.PostalCode, c.Email, c.PhoneNumber, c.Website, c.Individual, c.Banks,
		now, now,
	}
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, args...)
	} else {
		_, err = db.ExecContext(ctx, query, args...)
	}
	if err != nil {
		return "", err
	}
	return newID, nil
}

// MatchCompany finds an existing company for c using the same VAT/code/title
// matching as UpsertCompany but does NOT create one. Returns the matched id
// and whether a match was found. Used by the worker (P2-4.e): when an invoice
// has no VAT/code and no existing company matches, the invoice is left
// unassigned for a manual merge instead of creating a junk company.
func (s *Service) MatchCompany(ctx context.Context, c domain.Company, tx *sql.Tx) (string, bool, error) {
	if c.Title == "" {
		return "", false, nil
	}
	if c.VATCode != nil {
		n := normalizeVATCode(*c.VATCode)
		c.VATCode = &n
	}
	id, err := s.matchCompanyID(ctx, c, tx)
	if err != nil && err != sql.ErrNoRows {
		return "", false, err
	}
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return id, true, nil
}

// matchCompanyID resolves an existing company id by VAT code, then code, then a
// title+country fallback. Returns sql.ErrNoRows when nothing matches.
func (s *Service) matchCompanyID(ctx context.Context, c domain.Company, tx *sql.Tx) (string, error) {
	db := s.store.DB()
	exec := func(query string, args ...any) (string, error) {
		var id string
		var qErr error
		if tx != nil {
			qErr = tx.QueryRowContext(ctx, query, args...).Scan(&id)
		} else {
			qErr = db.QueryRowContext(ctx, query, args...).Scan(&id)
		}
		return id, qErr
	}
	if c.VATCode != nil && *c.VATCode != "" {
		if id, err := exec("SELECT id FROM companies WHERE org_id = ? AND vat_code = ?", c.OrgID, *c.VATCode); err == nil {
			return id, nil
		}
	}
	if c.Code != nil && *c.Code != "" {
		if c.Country != nil && *c.Country != "" {
			if id, err := exec("SELECT id FROM companies WHERE org_id = ? AND code = ? AND country = ?", c.OrgID, *c.Code, *c.Country); err == nil {
				return id, nil
			}
		} else {
			if id, err := exec("SELECT id FROM companies WHERE org_id = ? AND code = ?", c.OrgID, *c.Code); err == nil {
				return id, nil
			}
		}
	}
	if c.Title != "" {
		if c.Country != nil && *c.Country != "" {
			if id, err := exec("SELECT id FROM companies WHERE org_id = ? AND title = ? AND country = ?", c.OrgID, c.Title, *c.Country); err == nil {
				return id, nil
			}
		} else {
			if id, err := exec("SELECT id FROM companies WHERE org_id = ? AND title = ?", c.OrgID, c.Title); err == nil {
				return id, nil
			}
		}
	}
	return "", sql.ErrNoRows
}

// ListCompaniesForMerge returns a lightweight id+title list of the
// organization's companies excluding the given one, for the merge UI dropdown.
func (s *Service) ListCompaniesForMerge(ctx context.Context, orgID, excludeID string) ([]domain.Company, error) {
	rows, err := s.store.DB().QueryContext(ctx, `SELECT id, title FROM companies WHERE org_id = ? AND id != ? ORDER BY title ASC`, orgID, excludeID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Company
	for rows.Next() {
		var c domain.Company
		if err := rows.Scan(&c.ID, &c.Title); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
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

// MergeCompanies moves all invoice links (seller_company_id and
// buyer_company_id) from the source company to the target company within the
// same organization, then deletes the source company. The target's fields are
// kept; the source is removed. Used by the manual merge UI (P2-4.f) to clean
// up duplicate companies that the matcher created before VAT normalization.
func (s *Service) MergeCompanies(ctx context.Context, orgID, sourceID, targetID string) error {
	if sourceID == targetID {
		return errors.New("cannot merge a company with itself")
	}
	// Verify both companies exist and belong to the org.
	for _, id := range []string{sourceID, targetID} {
		c, err := s.GetCompany(ctx, id)
		if err != nil {
			return err
		}
		if c.OrgID != orgID {
			return sql.ErrNoRows
		}
	}

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `UPDATE invoices SET seller_company_id = ? WHERE seller_company_id = ? AND org_id = ?`, targetID, sourceID, orgID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE invoices SET buyer_company_id = ? WHERE buyer_company_id = ? AND org_id = ?`, targetID, sourceID, orgID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM companies WHERE id = ? AND org_id = ?`, sourceID, orgID); err != nil {
		return err
	}
	return tx.Commit()
}