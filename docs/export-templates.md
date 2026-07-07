# Export Templates

This guide explains how to create and edit **export templates** in Importinvoices, how the template engine works, what data is available to a template, and how the rendered output is delivered. It is the canonical reference for template authoring; the in-app help page at `/settings/export-templates/help` contains a condensed version of the same material.

> Looking for a quick answer? See the **Export** section of [QA.md](../QA.md) (Q 51 - Q 58f).

---

## 1. What export templates are

An export template is a reusable definition that turns selected invoices into a file (or files) understood by your accounting software, or into an HTTP request sent to an external API. The application ships with **16 prebuilt (system) templates** covering Lithuanian accounting systems (Rivile, i.SAF, Centas, Apskaita5, Debetas, Finvalda, Euroskaita, Lobasoft, Paulita, Pragma 3/4, StandardERP, Saikas, Agnum, Uniconta, plus a generic JSON template).

There are two export paths on the **Ready for export** tab:

- **Quick export** - built-in JSON / XML / CSV / TXT download, no template involved.
- **Accounting software** - uses a prebuilt or custom export template (this guide).

Templates are managed at **Settings -> Export templates** (`/settings/export-templates`).

---

## 2. Creating and editing a template

> Roles: **Operators** see templates read-only (list + preview). **Administrators** can create, edit, delete and favorite org-owned templates. System templates are read-only for everyone.

### 2.1 Create a new template

1. Go to **Settings -> Export templates** and click **Create template** (admin only).
2. The edit form has three tabs: **General info**, **Files** (or **API Request** for API type), and **Preview**.

### 2.2 General info fields

| Field | Required | Description |
|-------|----------|-------------|
| **Title** | yes | Display name shown in the export dropdown and template list. |
| **Description** | no | Free-text description shown on the template card. |
| **Country** | no | Country code (e.g. `LT`). Shown as a flag on the card. |
| **Website** | no | URL of the accounting software vendor. |
| **Type** | yes | **File export** or **API export**. Locked after creation. |
| **Active** | no | Inactive templates are hidden from the export menu. |
| **Favorite** | no | Favorite templates appear at the top of the list and the export dropdown. |

### 2.3 Files tab (File export)

A File template consists of one or more **files**. Each file has:

- a **filename** (e.g. `purchases.csv`, `sales.xml`, `Klientai.eip`) - the extension decides the download `Content-Type` and, when there are several non-empty files, the names inside the produced ZIP.
- a **content** textarea holding the template source (Go `text/template` syntax, see section 3).

Click **Add file** to append more files. Remove a file with the trash button on its card. File order is preserved in the ZIP.

### 2.4 API Request tab (API export)

An API template sends the exported data to an external HTTP endpoint instead of producing a download. The single request specification field accepts either:

- a **JSON** object:
  ```json
  {
    "URL": "https://accounting.example.com/api/invoices",
    "Method": "POST",
    "Headers": {
      "Authorization": "Bearer secret-token",
      "X-Tenant": "acme"
    },
    "Body": "{{jsonEscape .InvoiceType}}"
  }
  ```
- or just a plain URL string (`https://accounting.example.com/api/invoices`), in which case `Method` defaults to `POST`, no extra headers are set, and the body is the full payload JSON-marshaled.

`URL`, `Headers` values and `Body` are all rendered through the same Go `text/template` engine as file templates, so you can use `{{.InvoiceType}}`, `{{range .Invoices}}`, and every function listed in section 3.2 inside the body. If `Body` is empty, the whole `Payload` (section 4) is serialized to JSON and sent.

> Security: API export URLs are SSRF-validated. HTTPS is enforced and hostnames that resolve to loopback, private (RFC 1918), link-local, multicast, unspecified, or cloud-metadata IPs (e.g. `169.254.169.254`) are rejected, with a second check at TCP-connect time to block DNS rebinding. See QA.md Q 72a.

### 2.5 Preview tab

The **Preview** tab renders the template against a built-in **sample invoice** (a Lithuanian example: seller "Pardejas UAB", buyer "Pirkejas UAB", invoice SF 001, 100 + 21 EUR VAT). Use it to validate syntax before saving. The **Preview** button below the form re-renders live from the current editor content via `POST /api/v1/export/templates/preview` (admin only) - useful while iterating.

### 2.6 Save

Submit posts to `POST /settings/export-templates` (create) or `POST /settings/export-templates/{id}` (edit). You are then redirected back to the template list.

### 2.7 System templates

System templates (the 16 prebuilt ones) are shared across all organizations, loaded from the application's embedded template directory, and **read-only**: the Edit and Delete buttons are hidden, and a direct POST returns `403 Forbidden`. You cannot rename, modify, or remove them through the UI.

To customize a system template, **clone it manually**:

1. Open the system template's **Preview** page and switch to the **Template source** tab.
2. Copy the source of each file.
3. Back on the export-templates list, click **Create template**, choose **File export**, give it a title.
4. Add a file with the same filename, paste the copied source, and edit as needed.
5. Save and activate your new org-owned template.

There is no one-click clone button.

### 2.8 Favorite and delete

- The **star** icon on a card (admin only) toggles **Favorite**. Favorites sort to the top of the list and the export dropdown, and are marked with a star there too.
- **Delete** (admin only, non-system templates) asks for confirmation and removes the template and its files. This cannot be undone.

---

## 3. The template engine

### 3.1 Syntax - Go `text/template`

Templates use the standard Go [`text/template`](https://pkg.go.dev/text/template) engine (the same engine that renders the app's HTML). It is **not** Scriban, Handlebars, or Jinja. The most important constructs:

| Construct | Example | Meaning |
|-----------|---------|---------|
| Field access | `{{.InvoiceType}}` | Value of the `InvoiceType` field on the root payload. |
| Nested field | `{{.FromCompany.Title}}` | Field on a nested struct. |
| Range over a slice | `{{range $i, $inv := .Invoices}}...{{end}}` | Iterate; `$i` is the index, `$inv` the element. Index is optional: `{{range .Invoices}}`. |
| Parent reference | `{{$.InvoiceType}}` | Always refers to the root payload, even inside a `range`. |
| Conditional | `{{if .FromCompany}}...{{end}}` | Renders block only when truthy (non-nil pointer, non-empty string, non-zero number). |
| With | `{{with .FromCompany}}{{.Title}}{{end}}` | Sets `.` to `.FromCompany` inside the block if it is truthy. |
| Else | `{{if gt $item.VatAmount 0.0}}...{{else}}...{{end}}` | Standard if/else. |
| Whitespace trim | `{{- range ... -}}` | A leading `-` strips preceding whitespace; a trailing `-` strips following whitespace. Essential for clean CSV/XML. |
| Comparison | `{{gt $a $b}}`, `{{eq .Type "credit"}}` | `eq`, `ne`, `lt`, `le`, `gt`, `ge`. |
| Index | `{{index .Headers "Authorization"}}` | Map/slice indexing. |
| Parentheses | `{{(len $inv.Items)}}` | Group a sub-expression. |

> **Nil safety:** `FromCompany` and `ToCompany` are pointers. Always guard with `{{if $inv.FromCompany}}...{{end}}` before accessing their fields, or the template will error on invoices that have no seller/buyer assigned.

### 3.2 Custom functions

Beyond the standard built-ins, these functions are available in every template:

| Function | Signature | Example | Notes |
|----------|-----------|---------|-------|
| `xmlEscape` | `(s string) string` | `{{xmlEscape $inv.SeriesAndNumber}}` | Escapes `<`, `>`, `&`, `"` for XML/HTML text. |
| `csvEscape` | `(s string) string` | `{{csvEscape $inv.FromCompany.Code}}` | Wraps the value in double quotes and escapes embedded quotes (`""`). |
| `jsonEscape` | `(s string) string` | `"{{jsonEscape $inv.ID}}"` | Escapes a string for a JSON string literal (does not add surrounding quotes). |
| `cdata` | `(s string) string` | `{{cdata $item.Name}}` | Wraps the value in `<![CDATA[...]]>`. |
| `formatDate` | `(t time.Time, layout string) string` | `{{formatDate $inv.IssueDate "2006-01-02"}}` | Formats a `time.Time`. Returns empty string for the zero time. Layout uses Go's reference date `2006-01-02`. |
| `formatFloat` | `(v float64, decimals int) string` | `{{formatFloat $inv.AmountWithVat 2}}` | Fixed-decimal string (e.g. `121.00`). Uses shopspring/decimal. |
| `isZeroTime` | `(t time.Time) bool` | `{{if not (isZeroTime $inv.SupplyDate)}}...{{end}}` | True when a date was never set. |
| `companyField` | `(*Company, field string) string` | `{{companyField $inv.FromCompany "vat"}}` | Field accessor. `field` is one of: `title`, `code`, `vat`, `street`, `city`, `country`, `bank`. Returns `""` for a nil company. |
| `last` | `(i, n int) bool` | `{{if not (last $i (len $.Invoices))}},{{end}}` | True when `i` is the last index of a length-`n` sequence. Used to print commas between elements. |
| `add` | `(a, b int) int` | `{{add $j 1}}` | Integer addition. Handy for 1-based line numbers. |
| `mul` | `(a, b float64) float64` | `{{mul $item.AmountWithoutVat 100}}` | Float multiplication. |
| `div` | `(a, b float64) float64` | `{{div $a $b}}` | Float division. Returns `0` when the divisor is `0` (no panic). |
| `round` | `(v float64) int64` | `{{round (mul $item.VatAmount 100)}}` | Round half away from zero to `int64`. |
| `defaultStr` | `(fallback, value string) string` | `{{defaultStr "PVM1" $item.VatClassifier}}` | Returns `fallback` when `value` is blank (after trim). |
| `truncate` | `(s string, n int) string` | `{{truncate $inv.SeriesAndNumber 10}}` | Returns the first `n` characters of `s`. |

Standard `text/template` built-ins such as `len`, `index`, `print`/`printf`, `slice`, `or`, `and`, `not`, `call`, and the comparison functions are also available.

### 3.3 Patterns

**Comma between elements** (JSON arrays, CSV columns):

```
{{- range $j, $item := $inv.Items -}}
        { "name": "{{jsonEscape $item.Name}}" }{{if not (last $j (len $inv.Items))}},{{end}}
{{- end -}}
```

**Optional date** (omit when not set):

```
{{- if not (isZeroTime $inv.SupplyDate) -}}
<supply_date>{{formatDate $inv.SupplyDate "2006-01-02"}}</supply_date>
{{- end -}}
```

**Cents instead of euros** (some accounting imports want integer cents):

```
{{round (mul $item.AmountWithoutVat 100)}}
```

**Empty-file skip**: a file whose rendered content is empty (after trim) is omitted from the output entirely. This lets you use a guard like `{{- if gt (len .PurchasesInvoices) 0 -}}...{{- end -}}` so a "purchases" file disappears from the ZIP when there are no purchases. See section 5.

---

## 4. Data model

The root object passed to every template is `export.Payload` (referred to as `.` or `$` inside the template).

### 4.1 `Payload` (root `.`)

| Field | Go type | Template access | Description |
|-------|---------|-----------------|-------------|
| `Version` | `string` | `.Version` | Export schema version. |
| `ExportedAt` | `time.Time` | `.ExportedAt` | When the export run executed. |
| `InvoiceType` | `string` | `.InvoiceType` | The grouping chosen for this run: `"purchases"`, `"sales"`, or `"all"`. |
| `Companies` | `[]Company` | `.Companies` | All parties appearing in the selected invoices. |
| `Customers` | `[]Company` | `.Customers` | Parties that are buyers (`Internal: false`). |
| `Suppliers` | `[]Company` | `.Suppliers` | Parties that are sellers / your own org companies (`Internal: true`). |
| `Invoices` | `[]Invoice` | `.Invoices` | All selected invoices, in selection order. |
| `PurchasesInvoices` | `[]Invoice` | `.PurchasesInvoices` | Purchases bucket (see 4.5). |
| `SalesInvoices` | `[]Invoice` | `.SalesInvoices` | Sales bucket (see 4.5). |
| `InvoiceItems` | `[]Item` | `.InvoiceItems` | Flat list of every line item across all selected invoices. |
| `Now` | `time.Time` | `.Now` | Current time at render. |

> There is no separate "Organization" object in the template context. Your org companies are surfaced through `Suppliers` (sellers) and used internally to classify purchases vs sales.

### 4.2 `Invoice`

Accessed via `.Invoices`, `.PurchasesInvoices`, `.SalesInvoices`.

| Field | Go type | Description |
|-------|---------|-------------|
| `ID` | `string` | Internal invoice ID. |
| `ExternalID` | `string` | External system ID (currently always empty). |
| `FromCompany` | `*Company` | Seller (issuer). May be nil - guard with `if`. |
| `ToCompany` | `*Company` | Buyer (recipient). May be nil - guard with `if`. |
| `Filename` | `string` | Original uploaded file name. |
| `Checksum` | `string` | File checksum. |
| `URL` | `string` | Link to the original file when a base URL is configured; otherwise empty. |
| `SeriesAndNumber` | `string` | Human-readable invoice number, e.g. `SF 001`. |
| `Type` | `string` | `"invoice"` or `"credit"`. |
| `OriginalInvoicePublicID` | `string` | For credit notes: the referenced original invoice. |
| `IssueDate` | `time.Time` | Issue date. |
| `SupplyDate` | `time.Time` | Supply (delivery) date; may be zero. |
| `PaymentDueDate` | `time.Time` | Payment due date; may be zero. |
| `AmountWithoutVat` | `float64` | Invoice net total, **euros**. |
| `VatAmount` | `float64` | Invoice VAT total, euros. |
| `AmountWithVat` | `float64` | Invoice gross total, euros. |
| `Currency` | `string` | Currency code, e.g. `EUR`. |
| `Status` | `string` | Invoice status. |
| `Created` | `time.Time` | When the invoice record was created. |
| `Items` | `[]Item` | The invoice's line items. |

### 4.3 `Item` (line item)

| Field | Go type | Description |
|-------|---------|-------------|
| `Quantity` | `float64` | Quantity. **Negated for credit notes.** |
| `Name` | `string` | Line description. |
| `Code` | `string` | Product/service code (from the VAT classifier). |
| `UnitPrice` | `float64` | Unit price, euros. Derived when missing. |
| `AmountWithoutVat` | `float64` | Line net total, euros. |
| `VatAmount` | `float64` | Line VAT, euros. |
| `AmountWithVat` | `float64` | Line gross total, euros (equals stored `total_price`). |
| `VatRate` | `float64` | VAT rate as a number, e.g. `21`, `9`, `5`, `0`. Prefers the classifier's tariff. |
| `Currency` | `string` | Line currency (from the invoice). |
| `VatClassifier` | `string` | Classifier code, e.g. `PVM1`, `PVM2`, `PVM3`, `PVM5`, or a reverse-charge code. Falls back to `PVM1` when blank. |

### 4.4 `Company`

| Field | Go type | Description |
|-------|---------|-------------|
| `ID` | `string` | Internal company ID. |
| `ExternalID` | `string` | External system ID. |
| `Title` | `string` | Company name. |
| `Code` | `string` | Company / registration code. |
| `VATIdentificationNumber` | `string` | VAT number (without country prefix). |
| `Street` | `string` | Street address. |
| `City` | `string` | City. |
| `PostalCode` | `string` | Postal code. |
| `Country` | `string` | Country code, e.g. `LT`. |
| `PhoneNumber` | `string` | Phone. |
| `Email` | `string` | Email. |
| `Website` | `string` | Website. |
| `BankAccount` | `string` | Bank account number. |
| `Individual` | `bool` | True for a private person (not a legal entity). |
| `Internal` | `bool` | True when this company is your own organization (a seller in sales invoices). |

> Amounts are always **float euros** in templates. The database stores integer cents; the export layer converts. You do **not** need to divide by 100.

### 4.5 Purchases vs Sales grouping

The **Invoice grouping** selector on the export toolbar controls how selected invoices flow into the payload buckets. It does **not** filter which invoices are exported - it only affects where they land in the data:

| Selection | `.Invoices` | `.PurchasesInvoices` | `.SalesInvoices` |
|-----------|-------------|----------------------|------------------|
| **Purchases** | all selected | all selected | empty |
| **Sales** | all selected | empty | all selected |
| **All** | all selected | auto-classified per invoice | auto-classified per invoice |

When **All** is chosen, each invoice is classified by comparing its seller against your organization's companies: a match -> sales, otherwise -> purchases. With `All`, the purchases bucket also receives the sales invoices so purchase-side templates still see every invoice.

Most Lithuanian accounting templates iterate `.PurchasesInvoices` or `.SalesInvoices` rather than `.Invoices` - pick the bucket that matches the file you are producing.

### 4.6 Credit notes

When `Type == "credit"`, the export payload **negates** every monetary value: invoice header totals (net, VAT, gross) and every line's `Quantity`, `UnitPrice`, `AmountWithoutVat`, `VatAmount`, `AmountWithVat`. Accounting templates that sum the lines therefore reduce the totals automatically. The original stored amounts stay positive in the database; only the exported representation is negated. (See QA.md Q 58e.)

---

## 5. Output and delivery

### 5.1 File templates

`RenderTemplateFiles` renders each file, **drops any file whose rendered content is empty** (after trimming whitespace), then:

| Non-empty files | Result | `Content-Type` |
|-----------------|--------|----------------|
| 0 | `export.txt` (empty) | `text/plain` |
| 1 | the file, downloaded directly | from the filename extension |
| 2+ | a ZIP archive named `export_YYYYMMDD_HHMMSS.zip` | `application/zip` |

### 5.2 Supported extensions and content types

The download `Content-Type` is derived from the template file's extension:

| Extension | Content-Type |
|-----------|--------------|
| `.json` | `application/json` |
| `.xml` | `application/xml` |
| `.csv` | `text/csv` |
| `.txt` | `text/plain` |
| anything else (e.g. `.eip`) | `application/octet-stream` |

### 5.3 What is NOT supported

- **No PDF generation** from templates.
- **No HTML export** format.
- **No generic UBL renderer.** The Lobasoft template ships a UBL-flavored XML, but there is no built-in UBL output.

### 5.4 API templates

The rendered body is sent with `Content-Type: application/json` unless your `Headers` set one. The HTTP response (up to 1 MB) is returned to the caller as JSON; there is no file download. Status codes `>= 400` produce an error.

---

## 6. Examples

### 6.1 Minimal JSON file template

A single-file template named `export.json`. Adapted from the built-in `generic` template.

```gotemplate
{
  "version": "{{.Version}}",
  "exported_at": "{{formatDate .ExportedAt "2006-01-02T15:04:05Z07:00"}}",
  "invoice_type": "{{.InvoiceType}}",
  "documents": [
{{- range $i, $inv := .Invoices -}}
    {
      "id": "{{jsonEscape $inv.ID}}",
      "number": "{{jsonEscape $inv.SeriesAndNumber}}",
      "type": "{{jsonEscape $inv.Type}}",
      "issue_date": "{{formatDate $inv.IssueDate "2006-01-02"}}",
      "amount_excl_vat": {{formatFloat $inv.AmountWithoutVat 2}},
      "vat_amount": {{formatFloat $inv.VatAmount 2}},
      "amount_incl_vat": {{formatFloat $inv.AmountWithVat 2}},
      "seller": {
        "name": "{{if $inv.FromCompany}}{{jsonEscape $inv.FromCompany.Title}}{{end}}",
        "vat": "{{if $inv.FromCompany}}{{jsonEscape $inv.FromCompany.VATIdentificationNumber}}{{end}}"
      },
      "lines": [
{{- range $j, $item := $inv.Items -}}
        {
          "line_no": {{add $j 1}},
          "description": "{{jsonEscape $item.Name}}",
          "quantity": {{formatFloat $item.Quantity 4}},
          "unit_price": {{formatFloat $item.UnitPrice 4}},
          "amount_excl_vat": {{formatFloat $item.AmountWithoutVat 2}},
          "vat_rate": {{formatFloat $item.VatRate 2}}
        }{{if not (last $j (len $inv.Items))}},{{end}}
{{- end -}}
      ]
    }{{if not (last $i (len $.Invoices))}},{{end}}
{{- end -}}
  ]
}
```

Notes:
- `formatFloat` outputs bare numbers (no quotes) so the result is valid JSON.
- `jsonEscape` is used inside quoted strings; it does not add the quotes.
- `last` plus `{{if not ...}},{{end}}` inserts commas between array elements without a trailing one.
- `{{-` / `-}}` trim whitespace so the JSON stays compact.

### 6.2 CSV file template (purchases only, cents)

A `purchases.csv` file that disappears from the ZIP when there are no purchases. Adapted from the built-in `debetas` template.

```gotemplate
{{- if gt (len .PurchasesInvoices) 0 -}}
number,issue_date,seller_code,seller_name,currency,net_cents,vat_cents
{{- range $inv := .PurchasesInvoices -}}
{{- range $item := $inv.Items -}}
{{csvEscape $inv.SeriesAndNumber}},{{formatDate $inv.IssueDate "20060102"}},{{if $inv.FromCompany}}{{csvEscape $inv.FromCompany.Code}}{{end}},{{if $inv.FromCompany}}{{csvEscape $inv.FromCompany.Title}}{{end}},{{csvEscape $inv.Currency}},{{round (mul $item.AmountWithoutVat 100)}},{{round (mul $item.VatAmount 100)}}
{{- end -}}
{{- end -}}
{{- end -}}
```

Notes:
- The outer `{{- if gt (len .PurchasesInvoices) 0 -}}...{{- end -}}` makes the whole file empty when there are no purchases, so it is skipped by `RenderTemplateFiles`.
- `round (mul $amount 100)` converts euros to integer cents.
- `csvEscape` quotes each field so commas/quotes inside values are safe.

### 6.3 XML file template (suppliers master + purchases)

```gotemplate
<?xml version="1.0" encoding="UTF-8"?>
<export version="{{.Version}}" exported_at="{{formatDate .ExportedAt "2006-01-02T15:04:05Z07:00"}}">
  <suppliers>
{{- range $i, $s := .Suppliers -}}
    <supplier>
      <code>{{xmlEscape $s.Code}}</code>
      <name>{{xmlEscape $s.Title}}</name>
      <vat>{{xmlEscape $s.VATIdentificationNumber}}</vat>
      <country>{{xmlEscape $s.Country}}</country>
    </supplier>{{if not (last $i (len $.Suppliers))}}{{end}}
{{- end -}}
  </suppliers>
  <purchases>
{{- range $inv := .PurchasesInvoices -}}
    <invoice>
      <number>{{xmlEscape $inv.SeriesAndNumber}}</number>
      <issue_date>{{formatDate $inv.IssueDate "2006-01-02"}}</issue_date>
      <amount_excl_vat>{{formatFloat $inv.AmountWithoutVat 2}}</amount_excl_vat>
      <vat_amount>{{formatFloat $inv.VatAmount 2}}</vat_amount>
      <lines>
{{- range $item := $inv.Items -}}
        <line>
          <description>{{cdata $item.Name}}</description>
          <vat_classifier>{{xmlEscape (defaultStr "PVM1" $item.VatClassifier)}}</vat_classifier>
          <vat_rate>{{formatFloat $item.VatRate 2}}</vat_rate>
        </line>
{{- end -}}
      </lines>
    </invoice>
{{- end -}}
  </purchases>
</export>
```

Notes:
- `xmlEscape` for text content, `cdata` for free-text that may contain any characters.
- `defaultStr "PVM1" $item.VatClassifier` falls back to `PVM1` when the classifier is blank.
- `.Suppliers` is the deduplicated list of sellers; `.PurchasesInvoices` is the chosen bucket.

### 6.4 API template body

```json
{
  "URL": "https://accounting.example.com/api/invoices",
  "Method": "POST",
  "Headers": {
    "Authorization": "Bearer {{jsonEscape .InvoiceType}}-token",
    "Content-Type": "application/json"
  },
  "Body": "{\"batch\":\"{{jsonEscape .InvoiceType}}\",\"count\":{{len .Invoices}},\"exported_at\":\"{{formatDate .ExportedAt "2006-01-02T15:04:05Z07:00"}}\"}"
}
```

The `Body` string is itself a Go template that produces JSON. For anything non-trivial, prefer rendering a structured JSON document by iterating `.Invoices` (as in 6.1) inside `Body` rather than hand-escaping.

### 6.5 Fixed-width text (report.txt)

```gotemplate
{{- range $inv := .Invoices -}}
{{printf "%-15s" (truncate $inv.SeriesAndNumber 15)}} | {{formatDate $inv.IssueDate "2006-01-02"}} | {{printf "%10s" (formatFloat $inv.AmountWithVat 2)}} {{printf "%-3s" $inv.Currency}}
{{- end -}}
```

### 6.6 Multi-file ZIP (Purchases and Sales)

Create two files in the template: `purchases.csv` and `sales.csv`.

**File 1 (purchases.csv):**
```gotemplate
{{- if gt (len .PurchasesInvoices) 0 -}}
number,date,amount
{{- range $inv := .PurchasesInvoices -}}
{{csvEscape $inv.SeriesAndNumber}},{{formatDate $inv.IssueDate "2006-01-02"}},{{formatFloat $inv.AmountWithVat 2}}
{{- end -}}
{{- end -}}
```

**File 2 (sales.csv):**
```gotemplate
{{- if gt (len .SalesInvoices) 0 -}}
number,date,amount
{{- range $inv := .SalesInvoices -}}
{{csvEscape $inv.SeriesAndNumber}},{{formatDate $inv.IssueDate "2006-01-02"}},{{formatFloat $inv.AmountWithVat 2}}
{{- end -}}
{{- end -}}
```

### 6.7 Flat list of items

Uses `.InvoiceItems` to get every line across all selected invoices in one list.

```gotemplate
[
{{- range $i, $item := .InvoiceItems -}}
  {
    "name": "{{jsonEscape $item.Name}}",
    "total": {{formatFloat $item.AmountWithVat 2}},
    "vat_rate": {{formatFloat $item.VatRate 2}}
  }{{if not (last $i (len $.InvoiceItems))}},{{end}}
{{- end -}}
]
```

---

## 7. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Preview shows `parse template: ...` error | Unbalanced `{{ }}` or unknown function name. | Check that every `{{` has a matching `}}` and that the function exists in section 3.2. |
| Preview shows `render template: ...` error | Runtime failure, usually nil-pointer dereference on `FromCompany`/`ToCompany`. | Wrap field access in `{{if $inv.FromCompany}}...{{end}}`. |
| A file is missing from the ZIP | Its rendered content trimmed to empty. | This is expected when guarded by `{{if gt (len ...) 0}}`. Remove the guard or ensure data exists. |
| Numbers appear with quotes in JSON | You wrapped a `formatFloat` call in quotes. | Use `{{formatFloat $x 2}}` (no quotes) for numbers; reserve `"{{jsonEscape ...}}"` for strings. |
| Trailing comma breaks JSON/CSV | Missing `last` check between elements. | Add `{{if not (last $i (len $.Invoices))}},{{end}}` after each element. |
| API export fails with SSRF error | The URL is non-HTTPS or resolves to a private/loopback/metadata IP. | Use a public HTTPS endpoint. See QA.md Q 72a. |
| Edit/Delete buttons missing | The template is a system template, or you are signed in as an operator. | System templates are read-only; clone manually (section 2.7). Operators have read-only access by design. |
| VAT rate / code wrong in output | The line's VAT classifier is incorrect in the review step. | Fix the classifier on the invoice review page before exporting; the export follows the classifier (QA.md Q 58b). |
| Header totals disagree with lines | A reconciliation warning is added to the export result. | Review the invoice's amounts; the export still proceeds but the warning signals OCR/edit drift (QA.md Q 58f). |

---

## 8. Reference: where this lives in the code

| Concern | File |
|---------|------|
| Template engine + custom functions | `server/internal/export/template.go` |
| Render-to-file / ZIP / API execution | `server/internal/export/engine.go` |
| Payload, Invoice, Item, Company types | `server/internal/export/models.go` |
| Payload builder (grouping, credit-note negation) | `server/internal/export/context.go` |
| Quick formats (no template) | `server/internal/export/formats.go` |
| System templates (embedded) | `server/internal/export/system/*` |
| HTTP handlers (list, edit, preview, CRUD) | `server/internal/httpapi/export_template_handlers.go` |
| Routes | `server/internal/httpapi/router.go` |
| HTML templates | `server/internal/webui/templates/export_templates.html`, `export_template_edit.html`, `export_template_preview.html` |
| In-app help page | `/settings/export-templates/help` |

See also: [QA.md](../QA.md) **Export** section (Q 51 - Q 58f) and **Users & Roles** (Q 66a, Q 66d) for role-based access.
