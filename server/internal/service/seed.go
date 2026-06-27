package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/paleicikas/importinvoices/server/internal/export"
)

const SystemOrgID = "00000000-0000-0000-0000-000000000000"

func (s *Service) SeedExportTemplates(ctx context.Context) error {
	// Ensure system organization exists
	var exists bool
	err := s.store.DB().QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM organizations WHERE id = ?)", SystemOrgID).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		_, err = s.store.DB().ExecContext(ctx, `
			INSERT INTO organizations (id, title, created_at, updated_at)
			VALUES (?, ?, ?, ?)`,
			SystemOrgID, "System", time.Now().Unix(), time.Now().Unix())
		if err != nil {
			return fmt.Errorf("failed to create system organization: %w", err)
		}
	}

	systemTemplates := export.ListSystemTemplates()
	for _, st := range systemTemplates {
		err := s.seedTemplate(ctx, st)
		if err != nil {
			return fmt.Errorf("failed to seed template %s: %w", st.ID, err)
		}
	}

	return nil
}

func (s *Service) seedTemplate(ctx context.Context, st export.TemplateMeta) error {
	var exists bool
	err := s.store.DB().QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM export_templates WHERE id = ?)", st.ID).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	// Create new system template
	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO export_templates (
			id, org_id, type, title, description, country, website, active, is_system, is_favorite, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		st.ID, SystemOrgID, st.Type, st.Title, st.Description, st.Country, st.Website, st.Active, 1, 0, time.Now().Unix(), time.Now().Unix())
	if err != nil {
		return err
	}

	for _, f := range st.Files {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO export_template_files (id, template_id, filename, content, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), st.ID, f.Filename, f.Content, time.Now().Unix(), time.Now().Unix())
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
