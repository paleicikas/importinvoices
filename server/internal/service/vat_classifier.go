package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/vatcatalog"
)

func (s *Service) ListVatClassifiers(ctx context.Context, orgID string) ([]domain.VatClassifier, error) {
	rows, err := s.store.DB().QueryContext(ctx, `
		SELECT 
			id, org_id, country, code, tariff, description, example, 
			receiving_rule, issued_rule, active, reverse_charge, 
			purchase_account, include_in_isaf, created_at, updated_at
		FROM vat_classifiers 
		WHERE org_id = ?
		ORDER BY country ASC, code ASC`, orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var classifiers []domain.VatClassifier
	for rows.Next() {
		var vc domain.VatClassifier
		var createdAt, updatedAt int64
		if err := rows.Scan(
			&vc.ID, &vc.OrgID, &vc.Country, &vc.Code, &vc.Tariff, &vc.Description, &vc.Example,
			&vc.ReceivingRule, &vc.IssuedRule, &vc.Active, &vc.ReverseCharge,
			&vc.PurchaseAccount, &vc.IncludeInIsaf, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		vc.CreatedAt = time.Unix(createdAt, 0)
		vc.UpdatedAt = time.Unix(updatedAt, 0)
		classifiers = append(classifiers, vc)
	}
	return classifiers, nil
}

func (s *Service) GetVatClassifier(ctx context.Context, id string) (*domain.VatClassifier, error) {
	var vc domain.VatClassifier
	var createdAt, updatedAt int64
	err := s.store.DB().QueryRowContext(ctx, `
		SELECT 
			id, org_id, country, code, tariff, description, example, 
			receiving_rule, issued_rule, active, reverse_charge, 
			purchase_account, include_in_isaf, created_at, updated_at
		FROM vat_classifiers WHERE id = ?`, id).Scan(
		&vc.ID, &vc.OrgID, &vc.Country, &vc.Code, &vc.Tariff, &vc.Description, &vc.Example,
		&vc.ReceivingRule, &vc.IssuedRule, &vc.Active, &vc.ReverseCharge,
		&vc.PurchaseAccount, &vc.IncludeInIsaf, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	vc.CreatedAt = time.Unix(createdAt, 0)
	vc.UpdatedAt = time.Unix(updatedAt, 0)
	return &vc, nil
}

func (s *Service) CreateVatClassifier(ctx context.Context, vc *domain.VatClassifier) error {
	if vc.ID == "" {
		vc.ID = uuid.New().String()
	}
	now := time.Now().Unix()
	_, err := s.store.DB().ExecContext(ctx, `
		INSERT INTO vat_classifiers (
			id, org_id, country, code, tariff, description, example, 
			receiving_rule, issued_rule, active, reverse_charge, 
			purchase_account, include_in_isaf, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		vc.ID, vc.OrgID, vc.Country, vc.Code, vc.Tariff, vc.Description, vc.Example,
		vc.ReceivingRule, vc.IssuedRule, vc.Active, vc.ReverseCharge,
		vc.PurchaseAccount, vc.IncludeInIsaf, now, now)
	return err
}

func (s *Service) UpdateVatClassifier(ctx context.Context, vc *domain.VatClassifier) error {
	now := time.Now().Unix()
	_, err := s.store.DB().ExecContext(ctx, `
		UPDATE vat_classifiers SET 
			country = ?, code = ?, tariff = ?, description = ?, example = ?, 
			receiving_rule = ?, issued_rule = ?, active = ?, reverse_charge = ?, 
			purchase_account = ?, include_in_isaf = ?, updated_at = ?
		WHERE id = ? AND org_id = ?`,
		vc.Country, vc.Code, vc.Tariff, vc.Description, vc.Example,
		vc.ReceivingRule, vc.IssuedRule, vc.Active, vc.ReverseCharge,
		vc.PurchaseAccount, vc.IncludeInIsaf, now, vc.ID, vc.OrgID)
	return err
}

func (s *Service) DeleteVatClassifier(ctx context.Context, id, orgID string) error {
	res, err := s.store.DB().ExecContext(ctx, "DELETE FROM vat_classifiers WHERE id = ? AND org_id = ?", id, orgID)
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

func (s *Service) ImportCatalogCountry(ctx context.Context, orgID, countryCode string, missingOnly bool) error {
	catalog, err := vatcatalog.GetCatalog(countryCode)
	if err != nil {
		return err
	}

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()
	for _, entry := range catalog.Entries {
		if missingOnly {
			var exists bool
			err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM vat_classifiers WHERE org_id = ? AND country = ? AND code = ?)",
				orgID, catalog.CountryCode, entry.Code).Scan(&exists)
			if err != nil {
				return err
			}
			if exists {
				continue
			}
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO vat_classifiers (
				id, org_id, country, code, tariff, description, example, 
				receiving_rule, issued_rule, active, reverse_charge, 
				purchase_account, include_in_isaf, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(org_id, country, code) DO UPDATE SET
				tariff = excluded.tariff,
				description = excluded.description,
				example = excluded.example,
				receiving_rule = excluded.receiving_rule,
				issued_rule = excluded.issued_rule,
				reverse_charge = excluded.reverse_charge,
				purchase_account = excluded.purchase_account,
				include_in_isaf = excluded.include_in_isaf,
				updated_at = excluded.updated_at
			WHERE ? = 0`, // Only update if NOT missingOnly (i.e. if mode is 'all')
			uuid.New().String(), orgID, catalog.CountryCode, entry.Code, entry.Tariff, entry.Description, entry.Example,
			entry.ReceivingRule, entry.IssuedRule, true, entry.ReverseCharge,
			entry.PurchaseAccount, entry.IncludeInIsaf, now, now,
			func() int { if missingOnly { return 1 }; return 0 }())
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Service) ListAvailableCatalogCountries() ([]string, error) {
	return vatcatalog.ListCountries()
}

func (s *Service) GetCatalogPreview(countryCode string) (*vatcatalog.CountryCatalog, error) {
	return vatcatalog.GetCatalog(countryCode)
}
