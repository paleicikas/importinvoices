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
	svc    *Service
	client *http.Client
}

func NewWebhookService(svc *Service) *WebhookService {
	return &WebhookService{
		svc:    svc,
		client: export.SSRFSafeHTTPClient(15 * time.Second),
	}
}

// orgWebhooksSettingKey is the settings key under which organization-level
// webhook URLs are stored as a JSON map of event type -> URL.
const orgWebhooksSettingKey = "webhooks"

// GetWebhooks returns the organization-level webhook URL map. Returns an
// empty (non-nil) map when no webhooks are configured.
func (s *Service) GetWebhooks(ctx context.Context) (map[string]string, error) {
	raw, err := s.GetSetting(ctx, orgWebhooksSettingKey)
	if err != nil {
		return nil, err
	}
	urls := map[string]string{}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &urls); err != nil {
			return nil, err
		}
	}
	return urls, nil
}

// SetWebhooks persists the organization-level webhook URL map. Every non-empty
// URL is validated with export.ValidateWebhookURL (HTTPS-only + SSRF guard)
// at save time so bad/internal URLs cannot be persisted and later triggered.
func (s *Service) SetWebhooks(ctx context.Context, urls map[string]string) error {
	for event, u := range urls {
		if u == "" {
			continue
		}
		if err := export.ValidateWebhookURL(u); err != nil {
			return fmt.Errorf("webhook URL for %s: %w", event, err)
		}
	}
	raw, err := json.Marshal(urls)
	if err != nil {
		return err
	}
	return s.SetSetting(ctx, orgWebhooksSettingKey, string(raw))
}

func (s *WebhookService) SendWebhook(ctx context.Context, userID, eventType string, invoice *domain.Invoice) error {
	return s.SendInvoiceEvent(ctx, userID, eventType, invoice, "")
}

func (s *WebhookService) SendInvoiceEvent(ctx context.Context, userID, eventType string, invoice *domain.Invoice, baseURL string) error {
	// Webhooks are organization-level (configured in Settings → Webhooks by an
	// admin). userID is retained in the signature for call-site compatibility
	// but is not used to look up webhook URLs.
	urls, err := s.svc.GetWebhooks(ctx)
	if err != nil {
		return err
	}
	url, ok := urls[eventType]
	if !ok || url == "" {
		return nil
	}
	if err := export.ValidateWebhookURL(url); err != nil {
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

	resp, err := s.client.Do(req)
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
