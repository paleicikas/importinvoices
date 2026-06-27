package export

import "time"

// InvoiceType controls which template variable receives the invoice list.
type InvoiceType string

const (
	InvoiceTypePurchases InvoiceType = "purchases"
	InvoiceTypeSales     InvoiceType = "sales"
	InvoiceTypeAll       InvoiceType = "all"
)

// Options configures a single export run.
type Options struct {
	Format        string
	TemplateID    string
	InvoiceType   InvoiceType
	MarkExported  bool
	BaseURL       string
	MaxInvoices   int
	MaxOutputSize int64
}

const (
	DefaultMaxInvoices   = 5000
	DefaultMaxOutputSize = 50 * 1024 * 1024
)

// Company is a normalized party record for templates and integrations.
type Company struct {
	ID                      string  `json:"id"`
	ExternalID              string  `json:"external_id"`
	Title                   string  `json:"title"`
	Code                    string  `json:"code"`
	VATIdentificationNumber string  `json:"vat_identification_number"`
	Street                  string  `json:"street"`
	City                    string  `json:"city"`
	PostalCode              string  `json:"postal_code"`
	Country                 string  `json:"country"`
	PhoneNumber             string  `json:"phone_number"`
	Email                   string  `json:"email"`
	Website                 string  `json:"website"`
	BankAccount             string  `json:"bank_account"`
	Individual              bool    `json:"individual"`
	Internal                bool    `json:"internal"`
}

// Item is a normalized invoice line for export.
type Item struct {
	Quantity         float64 `json:"quantity"`
	Name             string  `json:"name"`
	Code             string  `json:"code"`
	UnitPrice        float64 `json:"unit_price"`
	AmountWithoutVat float64 `json:"amount_without_vat"`
	VatAmount        float64 `json:"vat_amount"`
	AmountWithVat    float64 `json:"amount_with_vat"`
	VatRate          float64 `json:"vat_rate"`
	Currency         string  `json:"currency"`
	VatClassifier    string  `json:"vat_classifier"`
}

// Invoice is the canonical export representation of an invoice document.
type Invoice struct {
	ID                      string    `json:"id"`
	ExternalID              string    `json:"external_id"`
	FromCompany             *Company  `json:"from_company"`
	ToCompany               *Company  `json:"to_company"`
	Filename                string    `json:"filename"`
	Checksum                string    `json:"checksum"`
	URL                     string    `json:"url"`
	SeriesAndNumber         string    `json:"series_and_number"`
	Type                    string    `json:"type"`
	OriginalInvoicePublicID string    `json:"original_invoice_public_id"`
	IssueDate               time.Time `json:"issue_date"`
	SupplyDate              time.Time `json:"supply_date"`
	PaymentDueDate          time.Time `json:"payment_due_date"`
	AmountWithoutVat        float64   `json:"amount_without_vat"`
	VatAmount               float64   `json:"vat_amount"`
	AmountWithVat           float64   `json:"amount_with_vat"`
	Currency                string    `json:"currency"`
	Status                  string    `json:"status"`
	Created                 time.Time `json:"created"`
	Items                   []Item    `json:"items"`
}

// Payload is the root object passed to templates and API integrations.
type Payload struct {
	Version            string    `json:"version"`
	ExportedAt         time.Time `json:"exported_at"`
	InvoiceType        string    `json:"invoice_type"`
	Companies          []Company `json:"companies"`
	Customers          []Company `json:"customers"`
	Suppliers          []Company `json:"suppliers"`
	Invoices           []Invoice `json:"invoices"`
	PurchasesInvoices  []Invoice `json:"purchases_invoices"`
	SalesInvoices      []Invoice `json:"sales_invoices"`
	InvoiceItems       []Item    `json:"invoice_items"`
	Now                time.Time `json:"now"`
}
