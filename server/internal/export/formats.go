package export

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

type xmlEnvelope struct {
	XMLName  xml.Name       `xml:"export"`
	Version  string         `xml:"version,attr"`
	Exported time.Time      `xml:"exported_at,attr"`
	Invoices []xmlInvoice   `xml:"invoices>invoice"`
}

type xmlInvoice struct {
	ID              string     `xml:"id,attr"`
	Number          string     `xml:"number"`
	Currency        string     `xml:"currency"`
	IssueDate       string     `xml:"issue_date"`
	SupplyDate      string     `xml:"supply_date"`
	PaymentDueDate  string     `xml:"payment_due_date"`
	AmountExclVat   float64    `xml:"amount_excl_vat"`
	VatAmount       float64    `xml:"vat_amount"`
	AmountInclVat   float64    `xml:"amount_incl_vat"`
	Seller          xmlParty   `xml:"seller"`
	Buyer           xmlParty   `xml:"buyer"`
	Items           []xmlItem  `xml:"items>item"`
}

type xmlParty struct {
	Name    string `xml:"name"`
	Code    string `xml:"code"`
	VAT     string `xml:"vat"`
	Street  string `xml:"street"`
	City    string `xml:"city"`
	Country string `xml:"country"`
}

type xmlItem struct {
	Description string  `xml:"description"`
	Quantity    float64 `xml:"quantity"`
	UnitPrice   float64 `xml:"unit_price"`
	AmountExcl  float64 `xml:"amount_excl_vat"`
	VatAmount   float64 `xml:"vat_amount"`
	AmountIncl  float64 `xml:"amount_incl_vat"`
	VatRate     float64 `xml:"vat_rate"`
}

// WriteQuickFormat writes a built-in export format without custom templates.
func WriteQuickFormat(format string, payload Payload, w io.Writer) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	case "xml":
		doc := xmlEnvelope{
			Version:  payload.Version,
			Exported: payload.ExportedAt,
		}
		for _, inv := range payload.Invoices {
			doc.Invoices = append(doc.Invoices, toXMLInvoice(inv))
		}
		enc := xml.NewEncoder(w)
		enc.Indent("", "  ")
		return enc.Encode(doc)
	case "csv":
		return writeCSV(payload, w)
	case "txt":
		return writeTXT(payload, w)
	default:
		return fmt.Errorf("unsupported export format: %s", format)
	}
}

func toXMLInvoice(inv Invoice) xmlInvoice {
	out := xmlInvoice{
		ID:             inv.ID,
		Number:         inv.SeriesAndNumber,
		Currency:       inv.Currency,
		IssueDate:      formatDate(inv.IssueDate, "2006-01-02"),
		SupplyDate:     formatDate(inv.SupplyDate, "2006-01-02"),
		PaymentDueDate: formatDate(inv.PaymentDueDate, "2006-01-02"),
		AmountExclVat:  inv.AmountWithoutVat,
		VatAmount:      inv.VatAmount,
		AmountInclVat:  inv.AmountWithVat,
	}
	if inv.FromCompany != nil {
		out.Seller = xmlParty{
			Name: inv.FromCompany.Title, Code: inv.FromCompany.Code,
			VAT: inv.FromCompany.VATIdentificationNumber, Street: inv.FromCompany.Street,
			City: inv.FromCompany.City, Country: inv.FromCompany.Country,
		}
	}
	if inv.ToCompany != nil {
		out.Buyer = xmlParty{
			Name: inv.ToCompany.Title, Code: inv.ToCompany.Code,
			VAT: inv.ToCompany.VATIdentificationNumber, Street: inv.ToCompany.Street,
			City: inv.ToCompany.City, Country: inv.ToCompany.Country,
		}
	}
	for _, item := range inv.Items {
		out.Items = append(out.Items, xmlItem{
			Description: item.Name, Quantity: item.Quantity, UnitPrice: item.UnitPrice,
			AmountExcl: item.AmountWithoutVat, VatAmount: item.VatAmount,
			AmountIncl: item.AmountWithVat, VatRate: item.VatRate,
		})
	}
	return out
}

func writeCSV(payload Payload, w io.Writer) error {
	cw := csv.NewWriter(w)
	header := []string{
		"invoice_id", "invoice_number", "invoice_date", "currency",
		"seller_name", "seller_code", "seller_vat",
		"buyer_name", "buyer_code", "buyer_vat",
		"line_description", "line_quantity", "line_unit_price",
		"line_amount_excl_vat", "line_vat", "line_amount_incl_vat", "line_vat_rate",
		"invoice_amount_excl_vat", "invoice_vat", "invoice_amount_incl_vat",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, inv := range payload.Invoices {
		if len(inv.Items) == 0 {
			if err := cw.Write(invoiceCSVRow(inv, Item{})); err != nil {
				return err
			}
			continue
		}
		for _, item := range inv.Items {
			if err := cw.Write(invoiceCSVRow(inv, item)); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}

func invoiceCSVRow(inv Invoice, item Item) []string {
	sellerName, sellerCode, sellerVAT := "", "", ""
	if inv.FromCompany != nil {
		sellerName, sellerCode, sellerVAT = inv.FromCompany.Title, inv.FromCompany.Code, inv.FromCompany.VATIdentificationNumber
	}
	buyerName, buyerCode, buyerVAT := "", "", ""
	if inv.ToCompany != nil {
		buyerName, buyerCode, buyerVAT = inv.ToCompany.Title, inv.ToCompany.Code, inv.ToCompany.VATIdentificationNumber
	}
	return []string{
		inv.ID, inv.SeriesAndNumber, formatDate(inv.IssueDate, "2006-01-02"), inv.Currency,
		sellerName, sellerCode, sellerVAT,
		buyerName, buyerCode, buyerVAT,
		item.Name, formatFloat(item.Quantity, 4), formatFloat(item.UnitPrice, 4),
		formatFloat(item.AmountWithoutVat, 2), formatFloat(item.VatAmount, 2),
		formatFloat(item.AmountWithVat, 2), formatFloat(item.VatRate, 2),
		formatFloat(inv.AmountWithoutVat, 2), formatFloat(inv.VatAmount, 2), formatFloat(inv.AmountWithVat, 2),
	}
}

func writeTXT(payload Payload, w io.Writer) error {
	for _, inv := range payload.Invoices {
		if _, err := fmt.Fprintf(w, "INVOICE %s\n", inv.SeriesAndNumber); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Date: %s\n", formatDate(inv.IssueDate, "2006-01-02")); err != nil {
			return err
		}
		if inv.FromCompany != nil {
			if _, err := fmt.Fprintf(w, "Seller: %s (%s)\n", inv.FromCompany.Title, inv.FromCompany.Code); err != nil {
				return err
			}
		}
		if inv.ToCompany != nil {
			if _, err := fmt.Fprintf(w, "Buyer: %s (%s)\n", inv.ToCompany.Title, inv.ToCompany.Code); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "Total: %s %s\n", formatFloat(inv.AmountWithVat, 2), inv.Currency); err != nil {
			return err
		}
		for _, item := range inv.Items {
			if _, err := fmt.Fprintf(w, "  - %s x%s @ %s = %s\n",
				item.Name, formatFloat(item.Quantity, 2), formatFloat(item.UnitPrice, 2),
				formatFloat(item.AmountWithVat, 2)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, strings.Repeat("-", 40)); err != nil {
			return err
		}
	}
	return nil
}
