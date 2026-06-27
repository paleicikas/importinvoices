package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/export"
)

type ExportParams struct {
	IDs          []string
	Format       string
	TemplateID   string
	InvoiceType  export.InvoiceType
	MarkExported bool
	BaseURL      string
	UserID       string
}

type ExportResult struct {
	ContentType string
	Filename    string
	APIResponse string
	IsAPI       bool
}

func (s *Service) ExportInvoices(ctx context.Context, params ExportParams, w io.Writer) (*ExportResult, error) {
	opts := export.Options{
		Format:        params.Format,
		TemplateID:    params.TemplateID,
		InvoiceType:   params.InvoiceType,
		MarkExported:  params.MarkExported,
		BaseURL:       params.BaseURL,
		MaxInvoices:   export.DefaultMaxInvoices,
		MaxOutputSize: export.DefaultMaxOutputSize,
	}
	if opts.Format == "" {
		opts.Format = "json"
	}
	if opts.InvoiceType == "" {
		opts.InvoiceType = export.InvoiceTypePurchases
	}

	invoices, itemsByInvoice, orgCompanies, err := s.loadExportData(ctx, params.IDs)
	if err != nil {
		return nil, err
	}
	if len(invoices) == 0 {
		return nil, fmt.Errorf("no invoices selected for export")
	}
	if len(invoices) > opts.MaxInvoices {
		return nil, fmt.Errorf("too many invoices selected (max %d)", opts.MaxInvoices)
	}

	payload := export.BuildPayload(invoices, itemsByInvoice, orgCompanies, opts.InvoiceType, opts.BaseURL)

	if params.TemplateID != "" {
		result, err := s.exportWithTemplate(ctx, params.TemplateID, payload, opts, w)
		if err != nil {
			return nil, err
		}
		if params.MarkExported {
			if err := s.MarkInvoicesExported(ctx, params.IDs); err != nil {
				return nil, err
			}
			if params.UserID != "" {
				s.NotifyInvoiceExported(ctx, params.UserID, params.IDs, params.BaseURL)
			}
		}
		return result, nil
	}

	filename := fmt.Sprintf("export_%s.%s", payload.ExportedAt.Format("20060102_150405"), strings.ToLower(opts.Format))
	contentType := contentTypeForFormat(opts.Format)
	if err := export.WriteQuickFormat(opts.Format, payload, w); err != nil {
		return nil, err
	}
	if params.MarkExported {
		if err := s.MarkInvoicesExported(ctx, params.IDs); err != nil {
			return nil, err
		}
		if params.UserID != "" {
			s.NotifyInvoiceExported(ctx, params.UserID, params.IDs, params.BaseURL)
		}
	}
	return &ExportResult{ContentType: contentType, Filename: filename}, nil
}

func (s *Service) exportWithTemplate(ctx context.Context, templateID string, payload export.Payload, opts export.Options, w io.Writer) (*ExportResult, error) {
	meta, files, apiReq, err := s.resolveExportTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if meta.Type == "api" {
		if apiReq == nil {
			return nil, fmt.Errorf("API export template has no request configuration")
		}
		status, body, err := export.ExecuteAPI(ctx, *apiReq, payload)
		if err != nil {
			return nil, fmt.Errorf("API export failed (%d): %w", status, err)
		}
		return &ExportResult{
			IsAPI:       true,
			APIResponse: body,
			ContentType: "application/json",
			Filename:    "api-export.json",
		}, nil
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("export template has no files")
	}
	contentType, filename, err := export.RenderTemplateFiles(files, payload, w)
	if err != nil {
		return nil, err
	}
	return &ExportResult{ContentType: contentType, Filename: filename}, nil
}

func (s *Service) loadExportData(ctx context.Context, ids []string) ([]domain.Invoice, map[string][]domain.InvoiceItem, []domain.Company, error) {
	var invoices []domain.Invoice
	itemsByInvoice := make(map[string][]domain.InvoiceItem)

	for _, id := range ids {
		inv, err := s.GetInvoiceForOrg(ctx, id)
		if err != nil {
			return nil, nil, nil, err
		}
		items, err := s.ListInvoiceItems(ctx, id)
		if err != nil {
			return nil, nil, nil, err
		}
		invoices = append(invoices, *inv)
		itemsByInvoice[id] = items
	}

	orgCompanies, err := s.loadExportCompanies(ctx)
	if err != nil {
		return invoices, itemsByInvoice, nil, nil
	}
	return invoices, itemsByInvoice, orgCompanies, nil
}

func (s *Service) resolveExportTemplate(ctx context.Context, templateID string) (export.TemplateMeta, []export.TemplateFile, *export.APIRequest, error) {
	tmpl, dbFiles, err := s.GetExportTemplate(ctx, templateID)
	if err != nil {
		return export.TemplateMeta{}, nil, nil, err
	}
	meta := export.TemplateMeta{
		ID:          tmpl.ID,
		Type:        tmpl.Type,
		Title:       tmpl.Title,
		Description: derefString(tmpl.Description),
		Country:     derefString(tmpl.Country),
		Website:     derefString(tmpl.Website),
		Active:      tmpl.Active,
		IsSystem:    tmpl.IsSystem,
	}
	files := make([]export.TemplateFile, 0, len(dbFiles))
	for _, f := range dbFiles {
		files = append(files, export.TemplateFile{Filename: f.Filename, Content: f.Content})
	}
	var apiReq *export.APIRequest
	if tmpl.Type == "api" && len(files) > 0 {
		req, parseErr := export.ParseAPIRequest(files[0].Content)
		if parseErr != nil {
			return meta, nil, nil, parseErr
		}
		apiReq = &req
	}
	return meta, files, apiReq, nil
}

func (s *Service) MarkInvoicesExported(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	orgID, err := s.organizationID(ctx)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	for _, id := range ids {
		if _, err := s.store.DB().ExecContext(ctx,
			"UPDATE invoices SET status = ?, updated_at = ? WHERE id = ? AND org_id = ? AND status = 'ready_for_export'",
			"exported", now, id, orgID,
		); err != nil {
			return err
		}
	}
	return nil
}

func contentTypeForFormat(format string) string {
	switch strings.ToLower(format) {
	case "json":
		return "application/json"
	case "xml":
		return "application/xml"
	case "csv":
		return "text/csv; charset=utf-8"
	case "txt":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
