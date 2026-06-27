package domain

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	WebhookUrls  *string   `json:"webhook_urls"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Organization struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Invoice struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	OrgID       string    `json:"org_id"`
	Status      string    `json:"status"`
	Filename    string    `json:"filename"`
	Checksum    string    `json:"checksum"`
	StoragePath string    `json:"storage_path"`
	PreviewPath *string   `json:"preview_path"`
	DuplicateOfID *string `json:"duplicate_of_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Parsed fields
	IsInvoice               *bool      `json:"is_invoice"`
	Type                    *int       `json:"type"`
	SeriesAndNumber         *string    `json:"series_and_number"`
	OriginalInvoicePublicID *string    `json:"original_invoice_public_id"`
	OcrText                 *string    `json:"ocr_text"`
	Currency                *string    `json:"currency"`
	IssueDate               *time.Time `json:"issue_date"`
	SupplyDate              *time.Time `json:"supply_date"`
	PaymentDueDate          *time.Time `json:"payment_due_date"`

	// Seller
	SellerName        *string `json:"seller_name"`
	SellerCode        *string `json:"seller_code"`
	SellerVAT         *string `json:"seller_vat"`
	SellerStreet      *string `json:"seller_street"`
	SellerCity        *string `json:"seller_city"`
	SellerCountry     *string `json:"seller_country"`
	SellerPostalCode  *string `json:"seller_postal_code"`
	SellerEmail       *string `json:"seller_email"`
	SellerPhoneNumber *string `json:"seller_phone_number"`
	SellerWebsite     *string `json:"seller_website"`
	SellerIndividual  *bool   `json:"seller_individual"`
	SellerBanks       *string `json:"seller_banks"`

	// Buyer
	BuyerName        *string `json:"buyer_name"`
	BuyerCode        *string `json:"buyer_code"`
	BuyerVAT         *string `json:"buyer_vat"`
	BuyerStreet      *string `json:"buyer_street"`
	BuyerCity        *string `json:"buyer_city"`
	BuyerCountry     *string `json:"buyer_country"`
	BuyerPostalCode  *string `json:"buyer_postal_code"`
	BuyerEmail       *string `json:"buyer_email"`
	BuyerPhoneNumber *string `json:"buyer_phone_number"`
	BuyerWebsite     *string `json:"buyer_website"`
	BuyerIndividual  *bool   `json:"buyer_individual"`
	BuyerBanks       *string `json:"buyer_banks"`

	AmountWithoutVat *float64 `json:"amount_without_vat"`
	VatAmount        *float64 `json:"vat_amount"`
	AmountWithVat    *float64 `json:"amount_with_vat"`

	ErrorMessage *string `json:"error_message"`
}

type InvoiceItem struct {
	ID            string   `json:"id"`
	InvoiceID     string   `json:"invoice_id"`
	Description   *string  `json:"description"`
	Quantity      *float64 `json:"quantity"`
	UnitPrice     *float64 `json:"unit_price"`
	TotalPrice    *float64 `json:"total_price"`
	VatAmount     *float64 `json:"vat_amount"`
	VatRate       *float64 `json:"vat_rate"`
	VatClassifier *string  `json:"vat_classifier"`
	CreatedAt     time.Time `json:"created_at"`
}

type Company struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Title       string    `json:"title"`
	Code        *string   `json:"code"`
	VATCode     *string   `json:"vat_code"`
	Street      *string   `json:"street"`
	City        *string   `json:"city"`
	Country     *string   `json:"country"`
	PostalCode  *string   `json:"postal_code"`
	Email       *string   `json:"email"`
	PhoneNumber *string   `json:"phone_number"`
	Website     *string   `json:"website"`
	Individual  *bool     `json:"individual"`
	Banks       *string   `json:"banks"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// View fields
	PurchasesCount int `json:"purchases_count"`
	SalesCount     int `json:"sales_count"`
}

type Setting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type VatClassifier struct {
	ID              string    `json:"id"`
	OrgID           string    `json:"org_id"`
	Country         string    `json:"country"`
	Code            string    `json:"code"`
	Tariff          float64   `json:"tariff"`
	Description     *string   `json:"description"`
	Example         *string   `json:"example"`
	ReceivingRule   *string   `json:"receiving_rule"`
	IssuedRule      *string   `json:"issued_rule"`
	Active          bool      `json:"active"`
	ReverseCharge   bool      `json:"reverse_charge"`
	PurchaseAccount *string   `json:"purchase_account"`
	IncludeInIsaf   bool      `json:"include_in_isaf"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
