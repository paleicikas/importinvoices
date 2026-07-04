package service

// Invoice list column indices — a single source of truth shared by the invoice
// service layer (sorting/filtering) and the MCP list_invoices tool. Previously
// the int->SQL-column map lived in invoice.go and a reverse name->int map lived
// in mcp.go, which had to be kept in sync by hand (and had already drifted in
// naming). Centralising them here removes that duplication.
const (
	InvoiceColCreatedAt        = 0
	InvoiceColSeriesAndNumber  = 1
	InvoiceColType             = 2
	InvoiceColIssueDate        = 3
	InvoiceColSupplyDate       = 4
	InvoiceColPaymentDueDate   = 5
	InvoiceColSellerName       = 6
	InvoiceColSellerCode       = 7
	InvoiceColSellerVat        = 8
	InvoiceColBuyerName        = 9
	InvoiceColBuyerCode        = 10
	InvoiceColBuyerVat         = 11
	InvoiceColAmountWithoutVat = 12
	InvoiceColVatAmount        = 13
	InvoiceColAmountWithVat    = 14
	InvoiceColCurrency         = 15
	InvoiceColStatus           = 16
	InvoiceColVatClassifier   = 35  // special: subquery on invoice_items.vat_classifier
	InvoiceColSellerComposite = 100 // special: seller_name/code/vat LIKE
	InvoiceColBuyerComposite  = 101 // special: buyer_name/code/vat LIKE
)

// InvoiceColumnSQL maps a column id to its underlying SQL column on the
// invoices table. The composite columns (100/101) are not plain SQL columns —
// they are expanded by the filter logic — so they are intentionally absent
// here; callers handle them explicitly.
var InvoiceColumnSQL = map[int]string{
	InvoiceColCreatedAt:        "created_at",
	InvoiceColSeriesAndNumber:  "series_and_number",
	InvoiceColType:             "type",
	InvoiceColIssueDate:        "issue_date",
	InvoiceColSupplyDate:       "supply_date",
	InvoiceColPaymentDueDate:   "payment_due_date",
	InvoiceColSellerName:       "seller_name",
	InvoiceColSellerCode:       "seller_code",
	InvoiceColSellerVat:        "seller_vat",
	InvoiceColBuyerName:        "buyer_name",
	InvoiceColBuyerCode:        "buyer_code",
	InvoiceColBuyerVat:         "buyer_vat",
	InvoiceColAmountWithoutVat: "amount_without_vat",
	InvoiceColVatAmount:        "vat_amount",
	InvoiceColAmountWithVat:    "amount_with_vat",
	InvoiceColCurrency:         "currency",
	InvoiceColStatus:           "status",
	InvoiceColVatClassifier:    "vat_classifier",
}

// InvoiceColumnIndexByName maps a public filter/sort name (used by the MCP API
// and UI query params) to its column id. "vat_codes" is the public alias for
// the vat_classifier column (the MCP schema exposes it under that name).
var InvoiceColumnIndexByName = map[string]int{
	"created_at":         InvoiceColCreatedAt,
	"series_and_number":  InvoiceColSeriesAndNumber,
	"type":               InvoiceColType,
	"issue_date":         InvoiceColIssueDate,
	"supply_date":        InvoiceColSupplyDate,
	"payment_due_date":   InvoiceColPaymentDueDate,
	"seller_name":        InvoiceColSellerName,
	"seller_code":        InvoiceColSellerCode,
	"seller_vat":         InvoiceColSellerVat,
	"buyer_name":         InvoiceColBuyerName,
	"buyer_code":         InvoiceColBuyerCode,
	"buyer_vat":          InvoiceColBuyerVat,
	"amount_without_vat": InvoiceColAmountWithoutVat,
	"vat_amount":         InvoiceColVatAmount,
	"amount_with_vat":    InvoiceColAmountWithVat,
	"currency":           InvoiceColCurrency,
	"status":             InvoiceColStatus,
	"vat_codes":          InvoiceColVatClassifier,
}

// InvoiceDateColumns are column ids whose filter values are date ranges
// (from,to) parsed as YYYY-MM-DD.
var InvoiceDateColumns = map[int]bool{
	InvoiceColCreatedAt:      true,
	InvoiceColIssueDate:      true,
	InvoiceColSupplyDate:     true,
	InvoiceColPaymentDueDate: true,
}

// InvoiceExactMatchColumns use "=" instead of "LIKE %...%" for filtering.
var InvoiceExactMatchColumns = map[string]bool{
	"status":            true,
	"type":              true,
	"buyer_code":        true,
	"seller_code":       true,
	"currency":          true,
	"series_and_number": true,
	"buyer_name":        true,
	"seller_name":       true,
}
