package service

import (
	"context"
	"fmt"
	"time"

	"github.com/paleicikas/importinvoices/server/internal/export"
)

type ExportTemplate struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	Country     *string   `json:"country"`
	Website     *string   `json:"website"`
	Active      bool      `json:"active"`
	IsSystem    bool      `json:"is_system"`
	IsFavorite  bool      `json:"is_favorite"`
	FileCount   int       `json:"file_count,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ExportTemplatePreview struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
	Error    string `json:"error,omitempty"`
}

type ExportTemplateFile struct {
	ID         string    `json:"id"`
	TemplateID string    `json:"template_id"`
	Filename   string    `json:"filename"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (s *Service) ListExportTemplates(ctx context.Context, orgID string) ([]ExportTemplate, error) {
	return s.listDBExportTemplates(ctx, orgID)
}

func (s *Service) listDBExportTemplates(ctx context.Context, orgID string) ([]ExportTemplate, error) {
	rows, err := s.store.DB().QueryContext(ctx, `
		SELECT 
			t.id, t.org_id, t.type, t.title, t.description, t.country, t.website,
			t.active, t.is_system, t.is_favorite, t.created_at, t.updated_at,
			(SELECT COUNT(*) FROM export_template_files f WHERE f.template_id = t.id) AS file_count
		FROM export_templates t
		WHERE t.is_system = 1 OR t.org_id = ?
		ORDER BY t.is_favorite DESC, t.title ASC`, orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var templates []ExportTemplate
	for rows.Next() {
		var t ExportTemplate
		var createdAt, updatedAt int64
		if err := rows.Scan(
			&t.ID, &t.OrgID, &t.Type, &t.Title, &t.Description, &t.Country, &t.Website,
			&t.Active, &t.IsSystem, &t.IsFavorite, &createdAt, &updatedAt, &t.FileCount,
		); err != nil {
			return nil, err
		}
		t.CreatedAt = time.Unix(createdAt, 0)
		t.UpdatedAt = time.Unix(updatedAt, 0)
		templates = append(templates, t)
	}
	return templates, nil
}

func (s *Service) GetExportTemplate(ctx context.Context, id string) (*ExportTemplate, []ExportTemplateFile, error) {
	var t ExportTemplate
	var createdAt, updatedAt int64
	err := s.store.DB().QueryRowContext(ctx, `
		SELECT 
			id, org_id, type, title, description, country, website, active, is_system, is_favorite, created_at, updated_at
		FROM export_templates WHERE id = ?`, id).Scan(
		&t.ID, &t.OrgID, &t.Type, &t.Title, &t.Description, &t.Country, &t.Website, &t.Active, &t.IsSystem, &t.IsFavorite, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("export template not found")
	}
	t.CreatedAt = time.Unix(createdAt, 0)
	t.UpdatedAt = time.Unix(updatedAt, 0)

	rows, err := s.store.DB().QueryContext(ctx, `
		SELECT id, template_id, filename, content, created_at, updated_at
		FROM export_template_files WHERE template_id = ?`, id)
	if err != nil {
		return &t, nil, nil
	}
	defer func() { _ = rows.Close() }()

	var files []ExportTemplateFile
	for rows.Next() {
		var f ExportTemplateFile
		var createdAt, updatedAt int64
		if err := rows.Scan(&f.ID, &f.TemplateID, &f.Filename, &f.Content, &createdAt, &updatedAt); err != nil {
			return nil, nil, err
		}
		f.CreatedAt = time.Unix(createdAt, 0)
		f.UpdatedAt = time.Unix(updatedAt, 0)
		files = append(files, f)
	}

	return &t, files, nil
}

func (s *Service) PreviewExportTemplate(ctx context.Context, id string) ([]ExportTemplatePreview, error) {
	_, files, err := s.GetExportTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	return previewTemplateFiles(files), nil
}

func (s *Service) PreviewTemplateFiles(files []ExportTemplateFile) []ExportTemplatePreview {
	return previewTemplateFiles(files)
}

func previewTemplateFiles(files []ExportTemplateFile) []ExportTemplatePreview {
	payload := export.SamplePayload()
	out := make([]ExportTemplatePreview, 0, len(files))
	for _, f := range files {
		p := ExportTemplatePreview{Filename: f.Filename}
		rendered, err := export.RenderTemplate(f.Filename, f.Content, payload)
		if err != nil {
			p.Error = err.Error()
		} else {
			p.Content = rendered
		}
		out = append(out, p)
	}
	return out
}

func (s *Service) CreateExportTemplate(ctx context.Context, t *ExportTemplate, files []ExportTemplateFile) error {
	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO export_templates (
			id, org_id, type, title, description, country, website, active, is_system, is_favorite, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.OrgID, t.Type, t.Title, t.Description, t.Country, t.Website, t.Active, t.IsSystem, t.IsFavorite, time.Now().Unix(), time.Now().Unix())
	if err != nil {
		return err
	}

	for _, f := range files {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO export_template_files (id, template_id, filename, content, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			f.ID, t.ID, f.Filename, f.Content, time.Now().Unix(), time.Now().Unix())
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Service) UpdateExportTemplate(ctx context.Context, t *ExportTemplate, files []ExportTemplateFile) error {
	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		UPDATE export_templates SET 
			type = ?, title = ?, description = ?, country = ?, website = ?, active = ?, is_favorite = ?, updated_at = ?
		WHERE id = ?`,
		t.Type, t.Title, t.Description, t.Country, t.Website, t.Active, t.IsFavorite, time.Now().Unix(), t.ID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM export_template_files WHERE template_id = ?", t.ID)
	if err != nil {
		return err
	}

	for _, f := range files {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO export_template_files (id, template_id, filename, content, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			f.ID, t.ID, f.Filename, f.Content, time.Now().Unix(), time.Now().Unix())
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Service) DeleteExportTemplate(ctx context.Context, id string) error {
	_, err := s.store.DB().ExecContext(ctx, "DELETE FROM export_templates WHERE id = ?", id)
	return err
}
