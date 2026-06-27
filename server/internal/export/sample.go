package export

import "time"

// SamplePayload returns example data for template preview and validation.
func SamplePayload() Payload {
	issue := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	supply := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	due := time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC)
	now := time.Date(2024, 4, 1, 12, 0, 0, 0, time.UTC)

	seller := Company{
		ID: "seller-1", Title: "Pardavėjas UAB", Code: "123456789",
		VATIdentificationNumber: "LT123456789", Street: "Gedimino g. 1",
		City: "Vilnius", PostalCode: "01103", Country: "LT",
		Email: "info@seller.lt", Internal: true,
	}
	buyer := Company{
		ID: "buyer-1", Title: "Pirkėjas UAB", Code: "987654321",
		VATIdentificationNumber: "LT987654321", Street: "Konstitucijos pr. 20",
		City: "Vilnius", PostalCode: "09308", Country: "LT",
	}

	inv := Invoice{
		ID: "sample-inv-1", Filename: "SF-001.pdf", SeriesAndNumber: "SF 001",
		Type: "invoice", Currency: "EUR", IssueDate: issue, SupplyDate: supply,
		PaymentDueDate: due, AmountWithoutVat: 100, VatAmount: 21, AmountWithVat: 121,
		Status: "ready_for_export", Created: issue, FromCompany: &seller, ToCompany: &buyer,
		Items: []Item{{
			Quantity: 1, Name: "Paslaugos teikimas", Code: "PVM1",
			UnitPrice: 100, AmountWithoutVat: 100, VatAmount: 21, AmountWithVat: 121,
			VatRate: 21, Currency: "EUR", VatClassifier: "PVM1",
		}},
	}

	return Payload{
		Version:           "1.0",
		ExportedAt:        now,
		InvoiceType:       "purchases",
		Now:               now,
		Invoices:          []Invoice{inv},
		PurchasesInvoices: []Invoice{inv},
		InvoiceItems:      inv.Items,
		Companies:         []Company{seller, buyer},
		Suppliers:         []Company{seller},
		Customers:         []Company{buyer},
	}
}
