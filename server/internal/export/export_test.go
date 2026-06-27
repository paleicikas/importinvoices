package export

import (
	"strings"
	"testing"
	"time"

	"github.com/paleicikas/importinvoices/server/internal/domain"
)

func str(v string) *string { return &v }
func f64(v float64) *float64 { return &v }
func tm(v time.Time) *time.Time { return &v }

func TestBuildPayload(t *testing.T) {
	issue := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	inv := domain.Invoice{
		ID: "inv-1", Filename: "invoice.pdf", Status: "ready_for_export",
		SeriesAndNumber: str("A-001"), Currency: str("EUR"), IssueDate: tm(issue),
		SellerName: str("Seller UAB"), SellerCode: str("123"), SellerVAT: str("LT123"),
		BuyerName: str("Buyer UAB"), BuyerCode: str("456"),
		AmountWithoutVat: f64(100), VatAmount: f64(21), AmountWithVat: f64(121),
		CreatedAt: issue,
	}
	items := map[string][]domain.InvoiceItem{
		"inv-1": {{
			Description: str("Service"), Quantity: f64(1), UnitPrice: f64(100),
			TotalPrice: f64(121), VatAmount: f64(21), VatRate: f64(21),
		}},
	}
	payload := BuildPayload([]domain.Invoice{inv}, items, nil, InvoiceTypePurchases, "http://localhost:8080")

	if len(payload.Invoices) != 1 {
		t.Fatalf("expected 1 invoice, got %d", len(payload.Invoices))
	}
	if len(payload.PurchasesInvoices) != 1 {
		t.Fatalf("expected 1 purchase invoice")
	}
	if len(payload.InvoiceItems) != 1 {
		t.Fatalf("expected 1 invoice item")
	}
	if payload.Invoices[0].FromCompany == nil || payload.Invoices[0].FromCompany.Title != "Seller UAB" {
		t.Fatalf("seller not mapped")
	}
}

func TestRenderGenericTemplate(t *testing.T) {
	payload := Payload{
		Version: "1.0", ExportedAt: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		InvoiceType: "purchases",
		Invoices: []Invoice{{
			ID: "1", SeriesAndNumber: "A-1", Currency: "EUR",
			AmountWithoutVat: 10, VatAmount: 2.1, AmountWithVat: 12.1,
			FromCompany: &Company{Title: "Seller", Code: "111"},
			Items:       []Item{{Name: "Line", Quantity: 1, UnitPrice: 10, AmountWithoutVat: 10, VatAmount: 2.1, AmountWithVat: 12.1}},
		}},
	}
	meta, ok := GetSystemTemplate("system_generic")
	if !ok {
		t.Fatal("system_generic template missing")
	}
	out, err := RenderTemplate(meta.Files[0].Filename, meta.Files[0].Content, payload)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if !strings.Contains(out, `"number": "A-1"`) && !strings.Contains(out, `"number":"A-1"`) {
		t.Fatalf("expected invoice number in output: %s", out)
	}
}

func TestValidateExternalURL(t *testing.T) {
	if err := ValidateExternalURL("http://localhost/hook"); err == nil {
		t.Fatal("expected localhost to be blocked")
	}
	if err := ValidateExternalURL("https://example.com/hook"); err != nil {
		t.Fatalf("expected public URL to pass: %v", err)
	}
}

func TestCSVEscape(t *testing.T) {
	if got := CSVEscape(`a"b,c`); got != `"a""b,c"` {
		t.Fatalf("unexpected csv escape: %q", got)
	}
}

func samplePayload() Payload {
	issue := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	seller := &Company{Title: "Seller UAB", Code: "123", VATIdentificationNumber: "LT123", Street: "Main", City: "Vilnius", Country: "LT", BankAccount: "LT123456"}
	buyer := &Company{Title: "Buyer UAB", Code: "456", VATIdentificationNumber: "LT456", Street: "Other", City: "Kaunas", Country: "LT"}
	return Payload{
		Version: "1.0", ExportedAt: issue, InvoiceType: "purchases", Now: issue,
		Companies:         []Company{*seller, *buyer},
		Customers:         []Company{*buyer},
		Suppliers:         []Company{*seller},
		Invoices: []Invoice{{
			ID: "inv-1234567890", SeriesAndNumber: "A-1", Currency: "EUR", IssueDate: issue,
			PaymentDueDate: issue.AddDate(0, 0, 14), AmountWithoutVat: 100, VatAmount: 21, AmountWithVat: 121,
			FromCompany: seller, ToCompany: buyer,
			Items: []Item{{Name: "Service", Code: "S1", Quantity: 1, UnitPrice: 100, AmountWithoutVat: 100, VatAmount: 21, AmountWithVat: 121, VatClassifier: "PVM1"}},
		}},
		PurchasesInvoices: []Invoice{{
			ID: "inv-1234567890", SeriesAndNumber: "A-1", Currency: "EUR", IssueDate: issue,
			AmountWithoutVat: 100, VatAmount: 21, AmountWithVat: 121,
			FromCompany: seller, ToCompany: buyer,
			Items: []Item{{Name: "Service", Quantity: 1, AmountWithoutVat: 100, VatAmount: 21, AmountWithVat: 121}},
		}},
		SalesInvoices: []Invoice{{
			ID: "inv-1234567890", SeriesAndNumber: "A-1", Currency: "EUR", IssueDate: issue,
			AmountWithoutVat: 100, VatAmount: 21, AmountWithVat: 121,
			FromCompany: seller, ToCompany: buyer,
			Items: []Item{{Name: "Service", Quantity: 1, AmountWithoutVat: 100, VatAmount: 21, AmountWithVat: 121, VatClassifier: "PVM1"}},
		}},
	}
}

func TestAllSystemTemplatesRender(t *testing.T) {
	payload := samplePayload()
	templates := ListSystemTemplates()
	if len(templates) < 10 {
		t.Fatalf("expected at least 10 system templates, got %d", len(templates))
	}
	for _, meta := range templates {
		if len(meta.Files) == 0 {
			t.Fatalf("%s has no template files", meta.ID)
		}
		for _, f := range meta.Files {
			out, err := RenderTemplate(f.Filename, f.Content, payload)
			if err != nil {
				t.Fatalf("%s/%s: %v", meta.ID, f.Filename, err)
			}
			if strings.TrimSpace(out) == "" {
				t.Fatalf("%s/%s: empty output", meta.ID, f.Filename)
			}
		}
	}
}
