package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/paleicikas/importinvoices/server/internal/domain"
	"google.golang.org/genai"
)

const defaultGeminiModel = "gemini-2.5-flash"

type GeminiProcessor struct {
	client *genai.Client
	model  string
}

func NewGeminiProcessor(apiKey, model string) (*GeminiProcessor, error) {
	if model == "" {
		model = defaultGeminiModel
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	return &GeminiProcessor{
		client: client,
		model:  model,
	}, nil
}

func (p *GeminiProcessor) Process(ctx context.Context, imagePaths []string, vatClassifiers []domain.VatClassifier) (*Result, error) {
	var parts []*genai.Part
	for _, path := range imagePaths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read image %s: %w", path, err)
		}
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: "image/jpeg",
				Data:     data,
			},
		})
	}

	contents := []*genai.Content{{
		Role:  genai.RoleUser,
		Parts: parts,
	}}

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemPrompt(vatClassifiers)}},
		},
		Tools: []*genai.Tool{{
			FunctionDeclarations: []*genai.FunctionDeclaration{{
				Name:        "ParseInvoice",
				Description: "Parses and validates a structured representation of an invoice document",
				Parameters:  geminiInvoiceSchema(),
			}},
		}},
		ToolConfig: &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigModeAny,
			},
		},
	}

	resp, err := p.client.Models.GenerateContent(ctx, p.model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("gemini error: %w", err)
	}

	calls := resp.FunctionCalls()
	if len(calls) == 0 {
		return nil, fmt.Errorf("no function calls in response")
	}

	call := calls[0]
	if call.Name != "ParseInvoice" {
		return nil, fmt.Errorf("unexpected function call: %s", call.Name)
	}

	argsJSON, err := json.Marshal(call.Args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal function arguments: %w", err)
	}

	log.Printf("Gemini tool arguments: %s", string(argsJSON))
	return parseToolResult(string(argsJSON))
}

func geminiInvoiceSchema() *genai.Schema {
	bankSchema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"bank_name":      {Type: genai.TypeString, Description: "Bank name"},
			"account_number": {Type: genai.TypeString, Description: "Bank account number"},
		},
	}

	itemSchema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"name":               {Type: genai.TypeString, Description: "The name of the item"},
			"code":               {Type: genai.TypeString, Description: "The code representing the item"},
			"quantity":           {Type: genai.TypeNumber, Description: "The quantity of the item"},
			"discount":           {Type: genai.TypeNumber, Description: "The discount of the item"},
			"discount_type":      {Type: genai.TypeString, Description: "The discount type of the item (0 - percentage, 1 - amount)", Enum: []string{"0", "1"}},
			"amount_without_vat": {Type: genai.TypeNumber, Description: "The total price of the item excluding VAT"},
			"vat_amount":         {Type: genai.TypeNumber, Description: "The VAT amount for the item"},
			"amount_with_vat":    {Type: genai.TypeNumber, Description: "The total price of the item including VAT"},
			"currency":           {Type: genai.TypeString, Description: "The currency code in which the item price is specified (ISO 4217)"},
			"unit_of_measure":    {Type: genai.TypeString, Description: "The unit of measure for the item (e.g., pcs, kg, m, h, etc.)"},
			"vat_classifier":     {Type: genai.TypeString, Description: "The VAT classifier code (e.g., 'PVM1', 'PVM2', 'PVM17') for the item."},
		},
	}

	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"is_invoice":                     {Type: genai.TypeBoolean, Description: "Indicates whether the document is an invoice or not"},
			"type":                           {Type: genai.TypeString, Description: "The type of the invoice (0 - invoice, 1 - credit invoice)", Enum: []string{"0", "1"}},
			"original_invoice_public_id":     {Type: genai.TypeString, Description: "The original invoice number for credit invoices"},
			"ocr_text":                       {Type: genai.TypeString, Description: "Text (OCR) formatted as Markdown language"},
			"series_and_number":              {Type: genai.TypeString, Description: "The series and number of the invoice"},
			"currency":                       {Type: genai.TypeString, Description: "Currency code (ISO 4217)"},
			"issue_date":                     {Type: genai.TypeString, Description: "The date (formatted: yyyy-MM-dd) when the invoice was issued"},
			"supply_date":                    {Type: genai.TypeString, Description: "The date (formatted: yyyy-MM-dd) when the goods or services were supplied"},
			"payment_due_date":               {Type: genai.TypeString, Description: "The due date (formatted: yyyy-MM-dd) for payment of the invoice"},
			"seller_company_name":            {Type: genai.TypeString, Description: "Seller company name"},
			"seller_company_code":            {Type: genai.TypeString, Description: "Seller company code"},
			"seller_vat_identification_number": {Type: genai.TypeString, Description: "Seller VAT identification number"},
			"seller_phone_number":            {Type: genai.TypeString, Description: "Seller phone number"},
			"seller_email":                   {Type: genai.TypeString, Description: "Seller email"},
			"seller_website":                 {Type: genai.TypeString, Description: "Seller website (e.g.: https://www.website.com)"},
			"seller_street":                  {Type: genai.TypeString, Description: "The street address of the company"},
			"seller_city":                    {Type: genai.TypeString, Description: "The city in which the company is located"},
			"seller_country":                 {Type: genai.TypeString, Description: "Country alpha-2 code"},
			"seller_postal_code":             {Type: genai.TypeString, Description: "The postal or ZIP code for the company's address"},
			"seller_banks":                   {Type: genai.TypeArray, Items: bankSchema},
			"seller_individual":              {Type: genai.TypeBoolean, Description: "Seller is individual"},
			"buyer_company_name":             {Type: genai.TypeString, Description: "Buyer company name"},
			"buyer_company_code":             {Type: genai.TypeString, Description: "Buyer company code"},
			"buyer_vat_identification_number": {Type: genai.TypeString, Description: "Buyer VAT identification number"},
			"buyer_phone_number":             {Type: genai.TypeString, Description: "Buyer phone number"},
			"buyer_email":                    {Type: genai.TypeString, Description: "Buyer email"},
			"buyer_website":                  {Type: genai.TypeString, Description: "Buyer website (e.g.: https://www.website.com)"},
			"buyer_street":                   {Type: genai.TypeString, Description: "The street address of the company"},
			"buyer_city":                     {Type: genai.TypeString, Description: "The city in which the company is located"},
			"buyer_country":                  {Type: genai.TypeString, Description: "Country alpha-2 code"},
			"buyer_postal_code":              {Type: genai.TypeString, Description: "The postal or ZIP code for the company's address"},
			"buyer_banks":                    {Type: genai.TypeArray, Items: bankSchema},
			"buyer_individual":               {Type: genai.TypeBoolean, Description: "Buyer is individual"},
			"items":                          {Type: genai.TypeArray, Items: itemSchema},
			"amount_without_vat":             {Type: genai.TypeNumber, Description: "The total amount excluding VAT"},
			"vat_amount":                     {Type: genai.TypeNumber, Description: "The total VAT amount"},
			"amount_with_vat":                {Type: genai.TypeNumber, Description: "The total amount including VAT"},
		},
		Required: []string{
			"ocr_text",
			"series_and_number",
			"currency",
			"issue_date",
			"seller_company_name",
			"buyer_company_name",
			"items",
			"amount_without_vat",
			"vat_amount",
			"amount_with_vat",
		},
	}
}
