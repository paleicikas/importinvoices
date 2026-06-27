package processor

import (
	"context"

	"github.com/paleicikas/importinvoices/server/internal/domain"
)

type Result struct {
	IsInvoice               bool    `json:"is_invoice"`
	Type                    string  `json:"type"` // 0 - Invoice, 1 - Credit Invoice
	OriginalInvoicePublicId string  `json:"original_invoice_public_id"`
	OcrText                 string  `json:"ocr_text"`
	SeriesAndNumber         string  `json:"series_and_number"`
	Currency                string  `json:"currency"`
	IssueDate               string  `json:"issue_date"`
	SupplyDate              string  `json:"supply_date"`
	PaymentDueDate          string  `json:"payment_due_date"`

	// Seller
	SellerCompanyName             string `json:"seller_company_name"`
	SellerCompanyCode            string `json:"seller_company_code"`
	SellerVatIdentificationNumber string `json:"seller_vat_identification_number"`
	SellerPhoneNumber             string `json:"seller_phone_number"`
	SellerEmail                   string `json:"seller_email"`
	SellerWebsite                 string `json:"seller_website"`
	SellerStreet                  string `json:"seller_street"`
	SellerCity                    string `json:"seller_city"`
	SellerCountry                 string `json:"seller_country"`
	SellerPostalCode              string `json:"seller_postal_code"`
	SellerBanks                   []Bank `json:"seller_banks"`
	SellerIndividual              bool   `json:"seller_individual"`

	// Buyer
	BuyerCompanyName             string `json:"buyer_company_name"`
	BuyerCompanyCode            string `json:"buyer_company_code"`
	BuyerVatIdentificationNumber string `json:"buyer_vat_identification_number"`
	BuyerPhoneNumber             string `json:"buyer_phone_number"`
	BuyerEmail                   string `json:"buyer_email"`
	BuyerWebsite                 string `json:"buyer_website"`
	BuyerStreet                  string `json:"buyer_street"`
	BuyerCity                    string `json:"buyer_city"`
	BuyerCountry                 string `json:"buyer_country"`
	BuyerPostalCode              string `json:"buyer_postal_code"`
	BuyerBanks                   []Bank `json:"buyer_banks"`
	BuyerIndividual              bool   `json:"buyer_individual"`

	Items            []Item  `json:"items"`
	AmountWithoutVat float64 `json:"amount_without_vat"`
	VatAmount        float64 `json:"vat_amount"`
	AmountWithVat    float64 `json:"amount_with_vat"`

	RawJSON string `json:"-"`
}

type Bank struct {
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
}

type Item struct {
	Name             string  `json:"name"`
	Code             string  `json:"code"`
	Quantity         float64 `json:"quantity"`
	Discount         float64 `json:"discount"`
	DiscountType     string  `json:"discount_type"` // 0 - Percentage, 1 - Amount
	AmountWithoutVat float64 `json:"amount_without_vat"`
	VatAmount        float64 `json:"vat_amount"`
	AmountWithVat    float64 `json:"amount_with_vat"`
	Currency         string  `json:"currency"`
	UnitOfMeasure    string  `json:"unit_of_measure"`
	VatClassifier    string  `json:"vat_classifier"`
}

type Processor interface {
	Process(ctx context.Context, imagePaths []string, vatClassifiers []domain.VatClassifier) (*Result, error)
}
