package webui

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/paleicikas/importinvoices/server/internal/reqctx"
	"github.com/paleicikas/importinvoices/server/internal/service"
)

type Renderer struct {
	tmpl       *template.Template
	translator *Translator
	funcs      template.FuncMap
}

func invoiceStatusBadgeClass(status string) string {
	switch status {
	case "processed":
		return "bg-success"
	case "failed":
		return "bg-danger"
	case "duplicate":
		return "bg-warning text-dark"
	case "ready_for_export":
		return "bg-primary"
	case "exported":
		return "bg-secondary"
	case "pending", "processing":
		return "bg-info"
	default:
		return "bg-info"
	}
}

func NewRenderer() (*Renderer, error) {
	translator, err := NewTranslator()
	if err != nil {
		return nil, err
	}

	// Column-label helpers shared by activeFilters and colName. Previously
	// each FuncMap entry held its own hand-synced map[int]string literal keyed
	// by raw magic ints (0..35, 100, 101); the canonical invoice columns now
	// derive from the service.InvoiceCol* named registry so the int<->label
	// mapping lives in one place. The seller/buyer detail columns (17-34) and
	// the company columns (0-6) are UI-only and stay local to this file.
	invoiceColumnLabel := func(lang string, col int) string {
		canonical := map[int]string{
			service.InvoiceColCreatedAt:        translator.T(lang, "Created"),
			service.InvoiceColSeriesAndNumber:  translator.T(lang, "Number"),
			service.InvoiceColType:             translator.T(lang, "Type"),
			service.InvoiceColIssueDate:        translator.T(lang, "Date"),
			service.InvoiceColSupplyDate:       translator.T(lang, "Service date"),
			service.InvoiceColPaymentDueDate:   translator.T(lang, "Payment date"),
			service.InvoiceColSellerName:       translator.T(lang, "Seller"),
			service.InvoiceColSellerCode:       translator.T(lang, "Seller code"),
			service.InvoiceColSellerVat:        translator.T(lang, "Seller VAT"),
			service.InvoiceColBuyerName:        translator.T(lang, "Buyer"),
			service.InvoiceColBuyerCode:        translator.T(lang, "Buyer code"),
			service.InvoiceColBuyerVat:         translator.T(lang, "Buyer VAT"),
			service.InvoiceColAmountWithoutVat: translator.T(lang, "Amount excl. VAT"),
			service.InvoiceColVatAmount:        translator.T(lang, "VAT"),
			service.InvoiceColAmountWithVat:    translator.T(lang, "Amount incl. VAT"),
			service.InvoiceColCurrency:         translator.T(lang, "Currency"),
			service.InvoiceColStatus:           translator.T(lang, "Status"),
			service.InvoiceColVatClassifier:    translator.T(lang, "VAT codes"),
			service.InvoiceColSellerComposite:  translator.T(lang, "Seller"),
			service.InvoiceColBuyerComposite:   translator.T(lang, "Buyer"),
		}
		if l, ok := canonical[col]; ok {
			return l
		}
		detail := map[int]string{
			17: translator.T(lang, "Seller street"),
			18: translator.T(lang, "Seller city"),
			19: translator.T(lang, "Seller country"),
			20: translator.T(lang, "Seller postal code"),
			21: translator.T(lang, "Seller email"),
			22: translator.T(lang, "Seller phone"),
			23: translator.T(lang, "Seller website"),
			24: translator.T(lang, "Seller physical person"),
			25: translator.T(lang, "Seller banks"),
			26: translator.T(lang, "Buyer street"),
			27: translator.T(lang, "Buyer city"),
			28: translator.T(lang, "Buyer country"),
			29: translator.T(lang, "Buyer postal code"),
			30: translator.T(lang, "Buyer email"),
			31: translator.T(lang, "Buyer phone"),
			32: translator.T(lang, "Buyer website"),
			33: translator.T(lang, "Buyer physical person"),
			34: translator.T(lang, "Buyer banks"),
		}
		return detail[col]
	}
	companyColumnLabel := func(lang string, col int) string {
		labels := map[int]string{
			0: translator.T(lang, "Company name"),
			1: translator.T(lang, "Code"),
			2: translator.T(lang, "VAT code"),
			3: translator.T(lang, "City"),
			4: translator.T(lang, "Country"),
			5: translator.T(lang, "Purchases"),
			6: translator.T(lang, "Sales"),
		}
		return labels[col]
	}
	columnLabel := func(lang string, col int, listURL string) string {
		if listURL == "/companies" {
			return companyColumnLabel(lang, col)
		}
		return invoiceColumnLabel(lang, col)
	}

	funcs := template.FuncMap{
		"T": func(lang, key string) string {
			return translator.T(lang, key)
		},
		"statusLabel": func(lang, status string) string {
			labels := map[string]string{
				"pending":          translator.T(lang, "Processing"),
				"processing":       translator.T(lang, "Processing"),
				"processed":        translator.T(lang, "Awaiting confirmation"),
				"ready_for_export": translator.T(lang, "Ready for export"),
				"exported":         translator.T(lang, "Exported"),
				"duplicate":        translator.T(lang, "Duplicate"),
				"failed":           translator.T(lang, "Error"),
			}
			if label, ok := labels[status]; ok {
				return label
			}
			return status
		},
		"statusBadgeClass": invoiceStatusBadgeClass,
		"gravatar": func(email string, size int) string {
			email = strings.ToLower(strings.TrimSpace(email))
			hash := md5.Sum([]byte(email))
			return fmt.Sprintf("https://www.gravatar.com/avatar/%s?s=%d&d=mp", hex.EncodeToString(hash[:]), size)
		},
		"derefFloat": func(f *float64) float64 {
			if f == nil {
				return 0
			}
			return *f
		},
		"centsToFloat": func(c *int64) float64 {
			if c == nil {
				return 0
			}
			return float64(*c) / 100.0
		},
		"derefInt": func(i *int) int {
			if i == nil {
				return 0
			}
			return *i
		},
		"derefString": func(s *string) string {
			if s == nil || *s == "null" {
				return ""
			}
			return *s
		},
		"split": func(s, sep string) []string {
			if s == "" {
				return nil
			}
			return strings.Split(s, sep)
		},
		"colFilterValue": func(filters map[int][]string, col int) string {
			if filters == nil || len(filters[col]) == 0 {
				return ""
			}
			return filters[col][0]
		},
		"colFilterValueAt": func(filters map[int][]string, col int, idx int) string {
			if filters == nil || len(filters[col]) <= idx {
				return ""
			}
			return filters[col][idx]
		},
		"colFilterActive": func(filters map[int][]string, col int) bool {
			if filters == nil {
				return false
			}
			for _, v := range filters[col] {
				if strings.TrimSpace(v) != "" {
					return true
				}
			}
			return false
		},
		"listFilterURL": func(baseURL string, col int, value string, currentFilters map[int][]string, search string, sortCol int, sortDir string, tab string) string {
			u, _ := url.Parse(baseURL)
			q := u.Query()
			if tab != "" {
				q.Set("tab", tab)
			}
			if search != "" {
				q.Set("q", search)
			}
			if sortCol != 0 {
				q.Set("sort", strconv.Itoa(sortCol))
			}
			if sortDir != "" {
				q.Set("dir", sortDir)
			}
			for c, vals := range currentFilters {
				if c == col {
					continue
				}
				for _, v := range vals {
					q.Add("f."+strconv.Itoa(c), v)
				}
			}
			q.Set("f."+strconv.Itoa(col), value)
			u.RawQuery = q.Encode()
			return u.String()
		},
		"listSortURL": func(baseURL string, col int, currentFilters map[int][]string, search string, currentSortCol int, currentSortDir string, tab string) string {
			u, _ := url.Parse(baseURL)
			q := u.Query()
			if tab != "" {
				q.Set("tab", tab)
			}
			if search != "" {
				q.Set("q", search)
			}
			for c, vals := range currentFilters {
				for _, v := range vals {
					q.Add("f."+strconv.Itoa(c), v)
				}
			}
			dir := "asc"
			if currentSortCol == col && currentSortDir == "asc" {
				dir = "desc"
			}
			q.Set("sort", strconv.Itoa(col))
			q.Set("dir", dir)
			u.RawQuery = q.Encode()
			return u.String()
		},
		"listResetURL": func(baseURL string) string {
			return baseURL
		},
		"hasActiveFilters": func(filters map[int][]string) bool {
			for _, vals := range filters {
				for _, v := range vals {
					if strings.TrimSpace(v) != "" {
						return true
					}
				}
			}
			return false
		},
		"activeFilters": func(lang string, filters map[int][]string, listURL string) []map[string]any {
			var out []map[string]any
			cols := make([]int, 0, len(filters))
			for c := range filters {
				cols = append(cols, c)
			}
			sort.Ints(cols)

			for _, col := range cols {
				vals, ok := filters[col]
				if !ok {
					continue
				}
				label := columnLabel(lang, col, listURL)
				if label == "" {
					continue // Skip unknown columns
				}
				for _, v := range vals {
					if strings.TrimSpace(v) != "" {
						displayValue := v
						if listURL == "/invoices" && col == service.InvoiceColStatus {
							// Inline status label logic
							statusLabels := map[string]string{
								"pending":          translator.T(lang, "Processing"),
								"processing":       translator.T(lang, "Processing"),
								"processed":        translator.T(lang, "Awaiting confirmation"),
								"ready_for_export": translator.T(lang, "Ready for export"),
								"exported":         translator.T(lang, "Exported"),
								"duplicate":        translator.T(lang, "Duplicate"),
								"failed":           translator.T(lang, "Error"),
							}
							if sl, ok := statusLabels[v]; ok {
								displayValue = sl
							}
						}
						out = append(out, map[string]any{
							"Col":   col,
							"Label": label,
							"Value": displayValue,
						})
					}
				}
			}
			return out
		},
		"colName": func(lang string, col int, listURL string) string {
			return columnLabel(lang, col, listURL)
		},
		"listFilterRemoveURL": func(baseURL string, col int, value string, currentFilters map[int][]string, search string, sortCol int, sortDir string, tab string) string {
			u, _ := url.Parse(baseURL)
			q := u.Query()
			if tab != "" {
				q.Set("tab", tab)
			}
			if search != "" {
				q.Set("q", search)
			}
			if sortCol != 0 {
				q.Set("sort", strconv.Itoa(sortCol))
			}
			if sortDir != "" {
				q.Set("dir", sortDir)
			}
			for c, vals := range currentFilters {
				for _, v := range vals {
					if c == col && v == value {
						continue
					}
					q.Add("f."+strconv.Itoa(c), v)
				}
			}
			u.RawQuery = q.Encode()
			return u.String()
		},
		"seq": func(start, end int) []int {
			var out []int
			for i := start; i <= end; i++ {
				out = append(out, i)
			}
			return out
		},
		"ne": func(a, b any) bool {
			return a != b
		},
		"slice": func(s string, start, end int) string {
			if len(s) < end {
				return s
			}
			return s[start:end]
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"countBanks": func(banksStr *string) int {
			if banksStr == nil || *banksStr == "" || *banksStr == "null" || *banksStr == "[]" {
				return 0
			}
			// Simple count by searching for account numbers or just parsing JSON
			var banks []any
			if err := json.Unmarshal([]byte(*banksStr), &banks); err == nil {
				return len(banks)
			}
			return 0
		},
		"flag": func(code string) template.HTML {
			if code == "" {
				return ""
			}
			code = strings.ToLower(code)
			return template.HTML(fmt.Sprintf(`<span class="fi fi-%s me-1 shadow-sm border rounded-1" title="%s"></span>`, code, strings.ToUpper(code)))
		},
	"upper": strings.ToUpper,
	"lower": strings.ToLower,
	// i18n helpers backed by the central Locales registry (locales.go).
	"flagClass":  FlagCode,
	"isRTL":      IsRTL,
	"bcp47":      BCP47,
	"nativeName": NativeName,
	"shortLabel": ShortLabel,
	"langURL": func(currentURL *url.URL, lang string) string {
			if currentURL == nil {
				return "?lang=" + url.QueryEscape(lang)
			}
			u := *currentURL
			q := u.Query()
			q.Set("lang", lang)
			u.RawQuery = q.Encode()
			return u.String()
		},
	}

	tmpl := template.New("").Funcs(funcs)

	tmpl, err = tmpl.ParseFS(TemplateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{tmpl: tmpl, translator: translator, funcs: funcs}, nil
}

func (r *Renderer) Render(w io.Writer, name string, data any) error {
	return r.tmpl.ExecuteTemplate(w, name, data)
}

func (r *Renderer) RenderStandalonePage(w http.ResponseWriter, req *http.Request, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	lang := r.translator.GetLanguage(req)
	if req.URL.Query().Has("lang") {
		r.translator.SetLanguageCookie(w, lang)
	}

	m, ok := data.(map[string]any)
	if !ok {
		m = make(map[string]any)
	}

	m["Lang"] = lang
	m["CurrentURL"] = req.URL
	m["Locales"] = Locales
	if m["CSRFToken"] == nil {
		if token, ok := reqctx.CSRFToken(req.Context()); ok {
			m["CSRFToken"] = token
		} else if c, err := req.Cookie("csrf_token"); err == nil {
			m["CSRFToken"] = c.Value
		}
	}

	if err := r.tmpl.ExecuteTemplate(w, name, m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (r *Renderer) GetTranslator() *Translator {
	return r.translator
}

func (r *Renderer) RenderPage(w http.ResponseWriter, req *http.Request, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	lang := r.translator.GetLanguage(req)
	if req.URL.Query().Has("lang") {
		r.translator.SetLanguageCookie(w, lang)
	}

	// If data is a map, inject common data
	m, ok := data.(map[string]any)
	if !ok {
		m = make(map[string]any)
	}

	m["Lang"] = lang
	m["CurrentURL"] = req.URL
	m["Locales"] = Locales
	if m["CSRFToken"] == nil {
		if token, ok := reqctx.CSRFToken(req.Context()); ok {
			m["CSRFToken"] = token
		} else if c, err := req.Cookie("csrf_token"); err == nil {
			m["CSRFToken"] = c.Value
		}
	}
	if u, ok := reqctx.User(req.Context()); ok {
		m["User"] = u
	}

	if o, ok := reqctx.Organization(req.Context()); ok {
		m["Organization"] = o
	}

	// Flash messages
	if c, err := req.Cookie("flash"); err == nil {
		m["Flash"] = c.Value
		// Clear flash cookie
		http.SetCookie(w, &http.Cookie{
			Name:   "flash",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
	}
	if c, err := req.Cookie("flash_type"); err == nil {
		m["FlashType"] = c.Value
		// Clear flash_type cookie
		http.SetCookie(w, &http.Cookie{
			Name:   "flash_type",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
	}

	// 1. Render the specific page content into a buffer
	var body bytes.Buffer
	if err := r.tmpl.ExecuteTemplate(&body, name, m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Put the rendered content into the map for the layout
	m["Content"] = template.HTML(body.String())

	// 3. Render the main layout
	if err := r.tmpl.ExecuteTemplate(w, "layout", m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
