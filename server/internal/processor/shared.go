package processor

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/paleicikas/importinvoices/server/internal/domain"
)
func systemPrompt(vatClassifiers []domain.VatClassifier) string {
	baseText := `You are an expert invoice data parser. Parse the document into structured JSON.

## Rules for string values:
- ALWAYS return the 'OcrText' property with the full extracted text.
- The 'OcrText' value should be a plain string (markdown formatted).
- DO NOT wrap string values in markdown code blocks like ` + "```json" + ` or ` + "```markdown" + `.
- Return dates in yyyy-MM-dd format.
- Return country codes in ISO 3166-1 alpha-2.
- Return currency codes in ISO 4217.
- If a value is unknown, return an empty string.`

	if len(vatClassifiers) == 0 {
		return baseText
	}

	var vatText strings.Builder
	vatText.WriteString("\n\n## VAT Classifier (VatClassifier) Assignment Rules:\n\n")
	vatText.WriteString("The VatClassifier field is CRITICAL for accounting and tax reporting (i-SAF). You MUST assign the correct VAT classifier code to each invoice item based on the rules below.\n\n")
	vatText.WriteString("### Available VAT Classifiers:\n\n")

	for _, vc := range vatClassifiers {
		if !vc.Active {
			continue
		}
		vatText.WriteString(fmt.Sprintf("- **Code: %s** | Tariff: %.2f%%", vc.Code, vc.Tariff))
		if vc.ReverseCharge {
			vatText.WriteString(" | ReverseCharge: YES")
		}
		if !vc.IncludeInIsaf {
			vatText.WriteString(" | ExcludedFromISAF: YES")
		}
		vatText.WriteString("\n")
		if vc.Description != nil {
			vatText.WriteString(fmt.Sprintf("    Description: %s\n", *vc.Description))
		}
		if vc.Example != nil {
			vatText.WriteString(fmt.Sprintf("    Examples: %s\n", *vc.Example))
		}
		if vc.ReceivingRule != nil {
			vatText.WriteString(fmt.Sprintf("    When Receiving: %s\n", *vc.ReceivingRule))
		}
		if vc.IssuedRule != nil {
			vatText.WriteString(fmt.Sprintf("    When Issuing: %s\n", *vc.IssuedRule))
		}
		if vc.PurchaseAccount != nil {
			vatText.WriteString(fmt.Sprintf("    PurchaseAccount: %s\n", *vc.PurchaseAccount))
		}
	}

	vatText.WriteString(`
### VAT Classification Decision Logic:

1. Identify the VAT rate from the invoice VAT summary section.
2. Identify the transaction type based on seller/buyer countries (domestic, intra-EU acquisition, or import/reverse-charge).
3. Match the line's VAT rate to a classifier above whose Tariff equals that rate, honoring the ReverseCharge flag and the When Receiving / When Issuing rules for the transaction type. Do NOT default to any particular code based on the rate alone.
4. If the rate does not match any classifier's Tariff, or the transaction type does not fit a classifier's rules, leave VatClassifier empty rather than guessing.
`)
	return baseText + vatText.String()
}

func normalizeDates(r *Result) {
	r.IssueDate = normalizeDate(r.IssueDate)
	r.SupplyDate = normalizeDate(r.SupplyDate)
	r.PaymentDueDate = normalizeDate(r.PaymentDueDate)
}

func normalizeDate(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	formats := []string{"2006-01-02", "02.01.2006", "02/01/2006", "2006/01/02"}
	for _, f := range formats {
		t, err := time.Parse(f, dateStr)
		if err == nil {
			return t.Format("2006-01-02")
		}
	}
	return dateStr
}

func parseToolResult(raw string) (*Result, error) {
	var result Result
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool arguments: %w", err)
	}
	normalizeDates(&result)
	result.RawJSON = raw
	return &result, nil
}
