package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/export"
)

type WebhookService struct {
	svc *Service
}

func NewWebhookService(svc *Service) *WebhookService {
	return &WebhookService{svc: svc}
}

func (s *WebhookService) SendWebhook(ctx context.Context, userID, eventType string, invoice *domain.Invoice) error {
	return s.SendInvoiceEvent(ctx, userID, eventType, invoice, "")
}

func (s *WebhookService) SendInvoiceEvent(ctx context.Context, userID, eventType string, invoice *domain.Invoice, baseURL string) error {
	user, err := s.svc.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if user.WebhookUrls == nil || *user.WebhookUrls == "" {
		return nil
	}

	var urls map[string]string
	if err := json.Unmarshal([]byte(*user.WebhookUrls), &urls); err != nil {
		return err
	}
	url, ok := urls[eventType]
	if !ok || url == "" {
		return nil
	}
	if err := export.ValidateExternalURL(url); err != nil {
		return fmt.Errorf("webhook URL for %s: %w", eventType, err)
	}

	items, _ := s.svc.ListInvoiceItems(ctx, invoice.ID)
	orgCompanies, _ := s.svc.loadExportCompanies(ctx)
	payload := export.BuildPayload(
		[]domain.Invoice{*invoice},
		map[string][]domain.InvoiceItem{invoice.ID: items},
		orgCompanies,
		export.InvoiceTypeAll,
		baseURL,
	)

	body, err := json.Marshal(map[string]any{
		"event_type": eventType,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"export":     payload,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "importinvoices/1.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook failed with status: %s", resp.Status)
	}
	return nil
}

func (s *Service) loadExportCompanies(ctx context.Context) ([]domain.Company, error) {
	org, err := s.GetOrganization(ctx)
	if err != nil || org == nil {
		return nil, err
	}
	return s.ListCompanies(ctx, org.ID, CompanyListParams{SortCol: 0, SortDir: "asc"})
}

func (s *Service) ListInvoiceItems(ctx context.Context, invoiceID string) ([]domain.InvoiceItem, error) {
	_, items, err := s.GetInvoice(ctx, invoiceID)
	return items, err
}

func (s *Service) NotifyInvoiceExported(ctx context.Context, userID string, ids []string, baseURL string) {
	for _, id := range ids {
		inv, _, err := s.GetInvoice(ctx, id)
		if err != nil {
			continue
		}
		_ = s.Webhook.SendInvoiceEvent(ctx, userID, "invoice.exported", inv, baseURL)
	}
}
