package processor

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type OpenAIProcessor struct {
	client *openai.Client
	model  string
}

func NewOpenAIProcessor(apiKey, model string) *OpenAIProcessor {
	if model == "" {
		model = openai.GPT4oMini
	}
	return &OpenAIProcessor{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

func (p *OpenAIProcessor) Process(ctx context.Context, imagePaths []string, vatClassifiers []domain.VatClassifier) (*Result, error) {
	var messages []openai.ChatCompletionMessage

	// System message with detailed prompt
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: systemPrompt(vatClassifiers),
	})

	// User message with images
	var multiContent []openai.ChatMessagePart
	for _, path := range imagePaths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read image %s: %w", path, err)
		}
		base64Image := base64.StdEncoding.EncodeToString(data)
		multiContent = append(multiContent, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL:    "data:image/jpeg;base64," + base64Image,
				Detail: openai.ImageURLDetailAuto,
			},
		})
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:         openai.ChatMessageRoleUser,
		MultiContent: multiContent,
	})

	// Define the tool (function)
	f := openai.FunctionDefinition{
		Name:        "ParseInvoice",
		Description: "Parses and validates a structured representation of an invoice document",
		Parameters:  p.getFunctionParameters(),
	}

	t := openai.Tool{
		Type:     openai.ToolTypeFunction,
		Function: &f,
	}

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:      p.model,
		Messages:   messages,
		Tools:      []openai.Tool{t},
		ToolChoice: "required", // Use string "required" to force tool use, or specify the function
	})

	if err != nil {
		return nil, fmt.Errorf("openai error: %w", err)
	}

	if len(resp.Choices) == 0 || len(resp.Choices[0].Message.ToolCalls) == 0 {
		return nil, fmt.Errorf("no tool calls in response")
	}

	toolCall := resp.Choices[0].Message.ToolCalls[0]
	log.Printf("OpenAI tool arguments: %s", toolCall.Function.Arguments)
	return parseToolResult(toolCall.Function.Arguments)
}

func (p *OpenAIProcessor) getFunctionParameters() jsonschema.Definition {
	return jsonschema.Definition{
		Type: jsonschema.Object,
		Properties: map[string]jsonschema.Definition{
			"is_invoice": {
				Type:        jsonschema.Boolean,
				Description: "Indicates whether the document is an invoice or not",
			},
			"type": {
				Type:        jsonschema.String,
				Description: "The type of the invoice (0 - invoice, 1 - credit invoice)",
				Enum:        []string{"0", "1"},
			},
			"original_invoice_public_id": {
				Type:        jsonschema.String,
				Description: "The original invoice number for credit invoices",
			},
			"ocr_text": {
				Type:        jsonschema.String,
				Description: "Text (OCR) formatted as Markdown language",
			},
			"series_and_number": {
				Type:        jsonschema.String,
				Description: "The series and number of the invoice",
			},
			"currency": {
				Type:        jsonschema.String,
				Description: "Currency code (ISO 4217)",
			},
			"issue_date": {
				Type:        jsonschema.String,
				Description: "The date (formatted: yyyy-MM-dd) when the invoice was issued",
			},
			"supply_date": {
				Type:        jsonschema.String,
				Description: "The date (formatted: yyyy-MM-dd) when the goods or services were supplied",
			},
			"payment_due_date": {
				Type:        jsonschema.String,
				Description: "The due date (formatted: yyyy-MM-dd) for payment of the invoice",
			},
			"seller_company_name": {
				Type:        jsonschema.String,
				Description: "Seller company name",
			},
			"seller_company_code": {
				Type:        jsonschema.String,
				Description: "Seller company code",
			},
			"seller_vat_identification_number": {
				Type:        jsonschema.String,
				Description: "Seller VAT identification number",
			},
			"seller_phone_number": {
				Type:        jsonschema.String,
				Description: "Seller phone number",
			},
			"seller_email": {
				Type:        jsonschema.String,
				Description: "Seller email",
			},
			"seller_website": {
				Type:        jsonschema.String,
				Description: "Seller website (e.g.: https://www.website.com)",
			},
			"seller_street": {
				Type:        jsonschema.String,
				Description: "The street address of the company",
			},
			"seller_city": {
				Type:        jsonschema.String,
				Description: "The city in which the company is located",
			},
			"seller_country": {
				Type:        jsonschema.String,
				Description: "Country alpha-2 code",
			},
			"seller_postal_code": {
				Type:        jsonschema.String,
				Description: "The postal or ZIP code for the company's address",
			},
			"seller_banks": {
				Type: jsonschema.Array,
				Items: &jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"bank_name": {
							Type:        jsonschema.String,
							Description: "Bank name",
						},
						"account_number": {
							Type:        jsonschema.String,
							Description: "Bank account number",
						},
					},
				},
			},
			"seller_individual": {
				Type:        jsonschema.Boolean,
				Description: "Seller is individual",
			},
			"buyer_company_name": {
				Type:        jsonschema.String,
				Description: "Buyer company name",
			},
			"buyer_company_code": {
				Type:        jsonschema.String,
				Description: "Buyer company code",
			},
			"buyer_vat_identification_number": {
				Type:        jsonschema.String,
				Description: "Buyer VAT identification number",
			},
			"buyer_phone_number": {
				Type:        jsonschema.String,
				Description: "Buyer phone number",
			},
			"buyer_email": {
				Type:        jsonschema.String,
				Description: "Buyer email",
			},
			"buyer_website": {
				Type:        jsonschema.String,
				Description: "Buyer website (e.g.: https://www.website.com)",
			},
			"buyer_street": {
				Type:        jsonschema.String,
				Description: "The street address of the company",
			},
			"buyer_city": {
				Type:        jsonschema.String,
				Description: "The city in which the company is located",
			},
			"buyer_country": {
				Type:        jsonschema.String,
				Description: "Country alpha-2 code",
			},
			"buyer_postal_code": {
				Type:        jsonschema.String,
				Description: "The postal or ZIP code for the company's address",
			},
			"buyer_banks": {
				Type: jsonschema.Array,
				Items: &jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"bank_name": {
							Type:        jsonschema.String,
							Description: "Bank name",
						},
						"account_number": {
							Type:        jsonschema.String,
							Description: "Bank account number",
						},
					},
				},
			},
			"buyer_individual": {
				Type:        jsonschema.Boolean,
				Description: "Buyer is individual",
			},
			"items": {
				Type: jsonschema.Array,
				Items: &jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"name": {
							Type:        jsonschema.String,
							Description: "The name of the item",
						},
						"code": {
							Type:        jsonschema.String,
							Description: "The code representing the item",
						},
						"quantity": {
							Type:        jsonschema.Number,
							Description: "The quantity of the item",
						},
						"discount": {
							Type:        jsonschema.Number,
							Description: "The discount of the item",
						},
						"discount_type": {
							Type:        jsonschema.String,
							Description: "The discount type of the item (0 - percentage, 1 - amount)",
							Enum:        []string{"0", "1"},
						},
						"amount_without_vat": {
							Type:        jsonschema.Number,
							Description: "The total price of the item excluding VAT",
						},
						"vat_amount": {
							Type:        jsonschema.Number,
							Description: "The VAT amount for the item",
						},
						"amount_with_vat": {
							Type:        jsonschema.Number,
							Description: "The total price of the item including VAT",
						},
						"currency": {
							Type:        jsonschema.String,
							Description: "The currency code in which the item price is specified (ISO 4217)",
						},
						"unit_of_measure": {
							Type:        jsonschema.String,
							Description: "The unit of measure for the item (e.g., pcs, kg, m, h, etc.)",
						},
						"vat_classifier": {
							Type:        jsonschema.String,
							Description: "The VAT classifier code (e.g., 'PVM1', 'PVM2', 'PVM17') for the item.",
						},
					},
				},
			},
			"amount_without_vat": {
				Type:        jsonschema.Number,
				Description: "The total amount excluding VAT",
			},
			"vat_amount": {
				Type:        jsonschema.Number,
				Description: "The total VAT amount",
			},
			"amount_with_vat": {
				Type:        jsonschema.Number,
				Description: "The total amount including VAT",
			},
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
