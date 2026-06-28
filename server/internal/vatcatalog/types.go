package vatcatalog

type CatalogEntry struct {
	Code            string   `json:"code"`
	Tariff          float64  `json:"tariff"`
	Description     string   `json:"description"`
	Example         string   `json:"example,omitempty"`
	ReceivingRule   string   `json:"receiving_rule,omitempty"`
	IssuedRule      string   `json:"issued_rule,omitempty"`
	ReverseCharge   bool     `json:"reverse_charge"`
	PurchaseAccount string   `json:"purchase_account,omitempty"`
	IncludeInIsaf   bool     `json:"include_in_isaf"`
}

type CountryCatalog struct {
	CountryName string         `json:"country_name"`
	CountryCode string         `json:"country_code"` // ISO-2
	Entries     []CatalogEntry `json:"entries"`
}
