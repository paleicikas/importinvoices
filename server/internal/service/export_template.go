package service

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/paleicikas/importinvoices/server/internal/export"
)

// SystemOrgID is the synthetic organization owner of system (shared) export
// templates. System templates are no longer seeded into the database (see
// 001_initial.sql and NEXT-7); they are loaded from the embed FS at runtime.
// The constant is retained so the service-layer ExportTemplate representation
// can report a stable owner for embed-sourced system templates.
const SystemOrgID = "00000000-0000-0000-0000-000000000000"

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
	OutputLabel string    `json:"output_label,omitempty"`
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

// ListExportTemplates returns the organization's export templates plus the
// shared system templates, ordered by favorite then title. System templates
// are loaded from the embed FS (NEXT-7); user templates come from the DB.
// limit <= 0 returns all rows; a positive limit caps the result (used by
// dropdowns that should not render an unbounded number of <option> elements
// into the page).
func (s *Service) ListExportTemplates(ctx context.Context, orgID string, limit int) ([]ExportTemplate, error) {
	dbTemplates, err := s.listUserExportTemplates(ctx, orgID)
	if err != nil {
		return nil, err
	}
	merged := append(systemTemplatesList(), dbTemplates...)
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].IsFavorite != merged[j].IsFavorite {
			return merged[i].IsFavorite
		}
		return merged[i].Title < merged[j].Title
	})
	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}
	return merged, nil
}

// systemTemplatesList converts the embed system templates into the
// service-layer ExportTemplate shape (no files inlined for the list view —
// only FileCount and OutputLabel, mirroring the DB list query).
func systemTemplatesList() []ExportTemplate {
	metas := export.ListSystemTemplates()
	out := make([]ExportTemplate, 0, len(metas))
	for _, meta := range metas {
		t := systemTemplateHeader(meta)
		t.FileCount = len(meta.Files)
		firstFilename := ""
		if len(meta.Files) > 0 {
			firstFilename = meta.Files[0].Filename
		}
		t.OutputLabel = outputLabelForTemplate(meta.Type, t.FileCount, firstFilename)
		out = append(out, t)
	}
	return out
}

// systemTemplateHeader builds the ExportTemplate metadata (no files) for an
// embed system template.
func systemTemplateHeader(meta export.TemplateMeta) ExportTemplate {
	t := ExportTemplate{
		ID:       meta.ID,
		OrgID:    SystemOrgID,
		Type:     meta.Type,
		Title:    meta.Title,
		Active:   meta.Active,
		IsSystem: true,
	}
	if meta.Description != "" {
		d := meta.Description
		t.Description = &d
	}
	if meta.Country != "" {
		c := meta.Country
		t.Country = &c
	}
	if meta.Website != "" {
		w := meta.Website
		t.Website = &w
	}
	return t
}

// systemTemplateToExport converts an embed system template into the full
// service-layer ExportTemplate (with files) used by Get/Preview/Export paths.
func systemTemplateToExport(meta export.TemplateMeta) (*ExportTemplate, []ExportTemplateFile) {
	t := systemTemplateHeader(meta)
	t.FileCount = len(meta.Files)
	firstFilename := ""
	if len(meta.Files) > 0 {
		firstFilename = meta.Files[0].Filename
	}
	t.OutputLabel = outputLabelForTemplate(meta.Type, t.FileCount, firstFilename)
	files := make([]ExportTemplateFile, 0, len(meta.Files))
	for _, f := range meta.Files {
		files = append(files, ExportTemplateFile{Filename: f.Filename, Content: f.Content})
	}
	return &t, files
}

func (s *Service) listUserExportTemplates(ctx context.Context, orgID string) ([]ExportTemplate, error) {
	query := `
		SELECT
			t.id, t.org_id, t.type, t.title, t.description, t.country, t.website,
			t.active, t.is_system, t.is_favorite, t.created_at, t.updated_at,
			(SELECT COUNT(*) FROM export_template_files f WHERE f.template_id = t.id) AS file_count,
			(SELECT f.filename FROM export_template_files f WHERE f.template_id = t.id ORDER BY f.filename ASC LIMIT 1) AS first_filename
		FROM export_templates t
		WHERE t.is_system = 0 AND t.org_id = ?
		ORDER BY t.is_favorite DESC, t.title ASC`
	rows, err := s.store.DB().QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var templates []ExportTemplate
	for rows.Next() {
		var t ExportTemplate
		var createdAt, updatedAt int64
		var firstFilename *string
		if err := rows.Scan(
			&t.ID, &t.OrgID, &t.Type, &t.Title, &t.Description, &t.Country, &t.Website,
			&t.Active, &t.IsSystem, &t.IsFavorite, &createdAt, &updatedAt, &t.FileCount, &firstFilename,
		); err != nil {
			return nil, err
		}
		t.CreatedAt = time.Unix(createdAt, 0)
		t.UpdatedAt = time.Unix(updatedAt, 0)
		if firstFilename != nil {
			t.OutputLabel = outputLabelForTemplate(t.Type, t.FileCount, *firstFilename)
		} else {
			t.OutputLabel = outputLabelForTemplate(t.Type, t.FileCount, "")
		}
		templates = append(templates, t)
	}
	return templates, nil
}

func outputLabelForTemplate(typ string, fileCount int, firstFilename string) string {
	if typ == "api" {
		return "API"
	}
	if fileCount > 1 {
		return "ZIP"
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(firstFilename)), ".")
	switch ext {
	case "json":
		return "JSON"
	case "xml":
		return "XML"
	case "csv":
		return "CSV"
	case "txt":
		return "TXT"
	case "eip":
		return "EIP"
	default:
		if ext != "" {
			return strings.ToUpper(ext)
		}
	}
	return ""
}

func (s *Service) GetExportTemplate(ctx context.Context, id string) (*ExportTemplate, []ExportTemplateFile, error) {
	if meta, ok := export.GetSystemTemplate(id); ok {
		t, files := systemTemplateToExport(meta)
		return t, files, nil
	}
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

	files, err := s.loadExportTemplateFiles(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return &t, files, nil
}

// GetExportTemplateForOrg returns the template only if it is a system template
// (loaded from the embed FS, shared across all orgs) or belongs to the
// organization resolved from the context. Cross-org reads of a non-system
// template return "export template not found" without revealing it exists.
func (s *Service) GetExportTemplateForOrg(ctx context.Context, id string) (*ExportTemplate, []ExportTemplateFile, error) {
	if meta, ok := export.GetSystemTemplate(id); ok {
		t, files := systemTemplateToExport(meta)
		return t, files, nil
	}
	orgID, err := s.organizationID(ctx)
	if err != nil {
		return nil, nil, err
	}
	var t ExportTemplate
	var createdAt, updatedAt int64
	err = s.store.DB().QueryRowContext(ctx, `
		SELECT
			id, org_id, type, title, description, country, website, active, is_system, is_favorite, created_at, updated_at
		FROM export_templates WHERE id = ? AND is_system = 0 AND org_id = ?`, id, orgID).Scan(
		&t.ID, &t.OrgID, &t.Type, &t.Title, &t.Description, &t.Country, &t.Website, &t.Active, &t.IsSystem, &t.IsFavorite, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("export template not found")
	}
	t.CreatedAt = time.Unix(createdAt, 0)
	t.UpdatedAt = time.Unix(updatedAt, 0)

	files, err := s.loadExportTemplateFiles(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return &t, files, nil
}

func (s *Service) loadExportTemplateFiles(ctx context.Context, id string) ([]ExportTemplateFile, error) {
	rows, err := s.store.DB().QueryContext(ctx, `
		SELECT id, template_id, filename, content, created_at, updated_at
		FROM export_template_files WHERE template_id = ?`, id)
	if err != nil {
		return nil, nil
	}
	defer func() { _ = rows.Close() }()

	var files []ExportTemplateFile
	for rows.Next() {
		var f ExportTemplateFile
		var createdAt, updatedAt int64
		if err := rows.Scan(&f.ID, &f.TemplateID, &f.Filename, &f.Content, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		f.CreatedAt = time.Unix(createdAt, 0)
		f.UpdatedAt = time.Unix(updatedAt, 0)
		files = append(files, f)
	}
	return files, nil
}

func (s *Service) PreviewExportTemplate(ctx context.Context, id string) ([]ExportTemplatePreview, error) {
	_, files, err := s.GetExportTemplateForOrg(ctx, id)
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
