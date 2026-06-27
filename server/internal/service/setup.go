package service

import (
	"context"
)

func (s *Service) NeedsSetup(ctx context.Context) (bool, error) {
	var count int
	err := s.store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *Service) Setup(ctx context.Context, orgTitle, adminName, adminEmail, adminPassword string) error {
	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Create Organization
	_, err = s.insertOrganization(ctx, tx, orgTitle)
	if err != nil {
		return err
	}

	// Create Admin User
	_, err = s.insertUser(ctx, tx, adminEmail, adminPassword, adminName)
	if err != nil {
		return err
	}

	return tx.Commit()
}
