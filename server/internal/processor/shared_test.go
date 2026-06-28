package processor

import (
	"strings"
	"testing"

	"github.com/paleicikas/importinvoices/server/internal/domain"
)

func TestNormalizeDate(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"2024-03-15", "2024-03-15"},
		{"15.03.2024", "2024-03-15"},
		{"15/03/2024", "2024-03-15"},
		{"2024/03/15", "2024-03-15"},
		{"not-a-date", "not-a-date"},
	}

	for _, tt := range tests {
		if got := normalizeDate(tt.in); got != tt.want {
			t.Errorf("normalizeDate(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseToolResultNormalizesDates(t *testing.T) {
	raw := `{
		"is_invoice": true,
		"type": "0",
		"ocr_text": "Invoice",
		"series_and_number": "A-1",
		"currency": "EUR",
		"issue_date": "15.03.2024",
		"supply_date": "2024-03-14",
		"payment_due_date": "31/03/2024",
		"seller_company_name": "Seller",
		"buyer_company_name": "Buyer",
		"items": [],
		"amount_without_vat": 100,
		"vat_amount": 21,
		"amount_with_vat": 121
	}`

	result, err := parseToolResult(raw)
	if err != nil {
		t.Fatalf("parseToolResult: %v", err)
	}
	if result.IssueDate != "2024-03-15" {
		t.Fatalf("IssueDate = %q", result.IssueDate)
	}
	if result.SupplyDate != "2024-03-14" {
		t.Fatalf("SupplyDate = %q", result.SupplyDate)
	}
	if result.PaymentDueDate != "2024-03-31" {
		t.Fatalf("PaymentDueDate = %q", result.PaymentDueDate)
	}
	if result.RawJSON != raw {
		t.Fatal("expected RawJSON to preserve original payload")
	}
}

func TestSystemPromptIncludesActiveVATClassifiers(t *testing.T) {
	desc := "Standard rate"
	example := "Domestic goods"
	receivingRule := "Apply to all incoming"
	issuedRule := "Apply to all outgoing"
	purchaseAccount := "6000"
	prompt := systemPrompt([]domain.VatClassifier{
		{
			Code:            "PVM1",
			Tariff:          21,
			Active:          true,
			Description:     &desc,
			Example:         &example,
			ReceivingRule:   &receivingRule,
			IssuedRule:      &issuedRule,
			PurchaseAccount: &purchaseAccount,
			IncludeInIsaf:   false,
		},
		{Code: "PVM2", Tariff: 9, Active: false},
	})

	if !strings.Contains(prompt, "PVM1") {
		t.Fatal("expected active classifier in prompt")
	}
	if strings.Contains(prompt, "PVM2") {
		t.Fatal("inactive classifier should be omitted")
	}
	if !strings.Contains(prompt, "Standard rate") {
		t.Fatal("expected classifier description in prompt")
	}
	if !strings.Contains(prompt, "Domestic goods") {
		t.Fatal("expected classifier examples in prompt")
	}
	if !strings.Contains(prompt, "Apply to all incoming") {
		t.Fatal("expected receiving rule in prompt")
	}
	if !strings.Contains(prompt, "Apply to all outgoing") {
		t.Fatal("expected issued rule in prompt")
	}
	if !strings.Contains(prompt, "6000") {
		t.Fatal("expected purchase account in prompt")
	}
	if !strings.Contains(prompt, "ExcludedFromISAF: YES") {
		t.Fatal("expected ExcludedFromISAF in prompt")
	}
}

func TestSystemPromptWithoutVATClassifiers(t *testing.T) {
	prompt := systemPrompt(nil)
	if strings.Contains(prompt, "VAT Classifier") {
		t.Fatal("expected no VAT section without classifiers")
	}
	if !strings.Contains(prompt, "expert invoice data parser") {
		t.Fatal("expected base prompt text")
	}
}
