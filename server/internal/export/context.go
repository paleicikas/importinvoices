package export

import (
	"strings"
	"time"

	"github.com/paleicikas/importinvoices/server/internal/domain"
)

type knownCompany struct {
	domain.Company
}

// BuildPayload converts domain invoices into a canonical export payload.
func BuildPayload(invoices []domain.Invoice, itemsByInvoice map[string][]domain.InvoiceItem, orgCompanies []domain.Company, invoiceType InvoiceType, baseURL string) Payload {
	known := indexKnownCompanies(orgCompanies)
	companyCache := make(map[string]Company)

	now := time.Now().UTC()
	out := Payload{
		Version:     "1.0",
		ExportedAt:  now,
		InvoiceType: string(invoiceType),
		Now:         now,
	}

	for _, inv := range invoices {
		exportInv := mapInvoice(inv, itemsByInvoice[inv.ID], known, companyCache, baseURL)
		out.Invoices = append(out.Invoices, exportInv)
		for _, item := range exportInv.Items {
			out.InvoiceItems = append(out.InvoiceItems, item)
		}

		switch classifyInvoice(inv, orgCompanies, invoiceType) {
		case InvoiceTypeSales:
			out.SalesInvoices = append(out.SalesInvoices, exportInv)
		default:
			out.PurchasesInvoices = append(out.PurchasesInvoices, exportInv)
		}
	}

	if invoiceType == InvoiceTypeAll {
		out.PurchasesInvoices = append(out.PurchasesInvoices, out.SalesInvoices...)
	}

	companySet := make(map[string]Company)
	for _, inv := range out.Invoices {
		if inv.FromCompany != nil {
			companySet[companyKey(inv.FromCompany.Code, inv.FromCompany.VATIdentificationNumber, inv.FromCompany.Title)] = *inv.FromCompany
		}
		if inv.ToCompany != nil {
			companySet[companyKey(inv.ToCompany.Code, inv.ToCompany.VATIdentificationNumber, inv.ToCompany.Title)] = *inv.ToCompany
		}
	}
	for _, c := range companySet {
		out.Companies = append(out.Companies, c)
		if c.Internal {
			out.Suppliers = append(out.Suppliers, c)
		} else {
			out.Customers = append(out.Customers, c)
		}
	}

	return out
}

func indexKnownCompanies(companies []domain.Company) map[string]knownCompany {
	out := make(map[string]knownCompany)
	for _, c := range companies {
		if code := derefString(c.Code); code != "" {
			out["code:"+code] = knownCompany{c}
		}
		if vat := derefString(c.VATCode); vat != "" {
			out["vat:"+vat] = knownCompany{c}
		}
		title := strings.TrimSpace(strings.ToLower(c.Title))
		if title != "" {
			out["title:"+title] = knownCompany{c}
		}
	}
	return out
}

func classifyInvoice(inv domain.Invoice, orgCompanies []domain.Company, requested InvoiceType) InvoiceType {
	if requested == InvoiceTypeSales || requested == InvoiceTypePurchases {
		return requested
	}
	for _, c := range orgCompanies {
		if matchesParty(c, inv.SellerName, inv.SellerCode, inv.SellerVAT) {
			return InvoiceTypeSales
		}
	}
	return InvoiceTypePurchases
}

func matchesParty(c domain.Company, name, code, vat *string) bool {
	if code != nil && c.Code != nil && strings.EqualFold(*code, *c.Code) {
		return true
	}
	if vat != nil && c.VATCode != nil && strings.EqualFold(*vat, *c.VATCode) {
		return true
	}
	if name != nil && strings.EqualFold(strings.TrimSpace(*name), strings.TrimSpace(c.Title)) {
		return true
	}
	return false
}

func mapInvoice(inv domain.Invoice, items []domain.InvoiceItem, known map[string]knownCompany, cache map[string]Company, baseURL string) Invoice {
	from := partyCompany(
		inv.SellerName, inv.SellerCode, inv.SellerVAT, inv.SellerStreet, inv.SellerCity,
		inv.SellerPostalCode, inv.SellerCountry, inv.SellerEmail, inv.SellerPhoneNumber,
		inv.SellerWebsite, inv.SellerIndividual, inv.SellerBanks, true, known, cache,
	)
	to := partyCompany(
		inv.BuyerName, inv.BuyerCode, inv.BuyerVAT, inv.BuyerStreet, inv.BuyerCity,
		inv.BuyerPostalCode, inv.BuyerCountry, inv.BuyerEmail, inv.BuyerPhoneNumber,
		inv.BuyerWebsite, inv.BuyerIndividual, inv.BuyerBanks, false, known, cache,
	)

	invType := "invoice"
	if inv.Type != nil && *inv.Type == 1 {
		invType = "credit"
	}

	exportItems := make([]Item, 0, len(items))
	for _, item := range items {
		qty := derefFloat(item.Quantity)
		unitPrice := derefFloat(item.UnitPrice)
		if unitPrice == 0 && qty > 0 {
			unitPrice = derefFloat(item.TotalPrice) / qty
		}
		exportItems = append(exportItems, Item{
			Quantity:         qty,
			Name:             derefString(item.Description),
			Code:             derefString(item.VatClassifier),
			UnitPrice:        unitPrice,
			AmountWithoutVat: derefFloat(item.TotalPrice) - derefFloat(item.VatAmount),
			VatAmount:        derefFloat(item.VatAmount),
			AmountWithVat:    derefFloat(item.TotalPrice),
			VatRate:          derefFloat(item.VatRate),
			Currency:         derefString(inv.Currency),
			VatClassifier:    derefString(item.VatClassifier),
		})
	}

	url := ""
	if baseURL != "" && inv.StoragePath != "" {
		url = strings.TrimRight(baseURL, "/") + "/invoices/" + inv.ID + "/file"
	}

	return Invoice{
		ID:                      inv.ID,
		FromCompany:             from,
		ToCompany:               to,
		Filename:                inv.Filename,
		Checksum:                inv.Checksum,
		URL:                     url,
		SeriesAndNumber:         derefString(inv.SeriesAndNumber),
		Type:                    invType,
		OriginalInvoicePublicID: derefString(inv.OriginalInvoicePublicID),
		IssueDate:               derefTime(inv.IssueDate),
		SupplyDate:              derefTime(inv.SupplyDate),
		PaymentDueDate:          derefTime(inv.PaymentDueDate),
		AmountWithoutVat:        derefFloat(inv.AmountWithoutVat),
		VatAmount:               derefFloat(inv.VatAmount),
		AmountWithVat:           derefFloat(inv.AmountWithVat),
		Currency:                derefString(inv.Currency),
		Status:                  inv.Status,
		Created:                 inv.CreatedAt,
		Items:                   exportItems,
	}
}

func partyCompany(
	name, code, vat, street, city, postal, country, email, phone, website *string,
	individual *bool, banks *string, internal bool,
	known map[string]knownCompany, cache map[string]Company,
) *Company {
	title := derefString(name)
	c := Company{
		Title:                   title,
		Code:                    derefString(code),
		VATIdentificationNumber: derefString(vat),
		Street:                  derefString(street),
		City:                    derefString(city),
		PostalCode:              derefString(postal),
		Country:                 derefString(country),
		Email:                   derefString(email),
		PhoneNumber:             derefString(phone),
		Website:                 derefString(website),
		BankAccount:             derefString(banks),
		Individual:              derefBool(individual),
		Internal:                internal,
	}

	key := companyKey(c.Code, c.VATIdentificationNumber, c.Title)
	if existing, ok := cache[key]; ok {
		return &existing
	}
	if match, ok := known[key]; ok {
		c.ID = match.ID
		c.ExternalID = derefString(match.Code)
		if c.Title == "" {
			c.Title = match.Title
		}
		if c.Code == "" {
			c.Code = derefString(match.Code)
		}
		if c.VATIdentificationNumber == "" {
			c.VATIdentificationNumber = derefString(match.VATCode)
		}
		if c.Street == "" {
			c.Street = derefString(match.Street)
		}
		if c.City == "" {
			c.City = derefString(match.City)
		}
		if c.PostalCode == "" {
			c.PostalCode = derefString(match.PostalCode)
		}
		if c.Country == "" {
			c.Country = derefString(match.Country)
		}
		if c.Email == "" {
			c.Email = derefString(match.Email)
		}
		if c.PhoneNumber == "" {
			c.PhoneNumber = derefString(match.PhoneNumber)
		}
		if c.Website == "" {
			c.Website = derefString(match.Website)
		}
		if c.BankAccount == "" {
			c.BankAccount = derefString(match.Banks)
		}
		if match.Individual != nil {
			c.Individual = *match.Individual
		}
	} else {
		c.ID = key
	}

	cache[key] = c
	copy := c
	return &copy
}
