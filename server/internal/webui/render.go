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
	"strconv"
	"strings"

	"github.com/paleicikas/importinvoices/server/internal/reqctx"
)

type Renderer struct {
	tmpl       *template.Template
	translator *Translator
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

	tmpl := template.New("").Funcs(template.FuncMap{
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
			columnLabels := map[int]string{
				0:   translator.T(lang, "Created"),
				1:   translator.T(lang, "Number"),
				2:   translator.T(lang, "Type"),
				3:   translator.T(lang, "Date"),
				4:   translator.T(lang, "Service date"),
				5:   translator.T(lang, "Payment date"),
				6:   translator.T(lang, "Seller"),
				7:   translator.T(lang, "Seller code"),
				8:   translator.T(lang, "Seller VAT"),
				9:   translator.T(lang, "Buyer"),
				10:  translator.T(lang, "Buyer code"),
				11:  translator.T(lang, "Buyer VAT"),
				12:  translator.T(lang, "Amount excl. VAT"),
				13:  translator.T(lang, "VAT"),
				14:  translator.T(lang, "Amount incl. VAT"),
				15:  translator.T(lang, "Currency"),
				16:  translator.T(lang, "Status"),
				17:  translator.T(lang, "Seller street"),
				18:  translator.T(lang, "Seller city"),
				19:  translator.T(lang, "Seller country"),
				20:  translator.T(lang, "Seller postal code"),
				21:  translator.T(lang, "Seller email"),
				22:  translator.T(lang, "Seller phone"),
				23:  translator.T(lang, "Seller website"),
				24:  translator.T(lang, "Seller physical person"),
				25:  translator.T(lang, "Seller banks"),
				26:  translator.T(lang, "Buyer street"),
				27:  translator.T(lang, "Buyer city"),
				28:  translator.T(lang, "Buyer country"),
				29:  translator.T(lang, "Buyer postal code"),
				30:  translator.T(lang, "Buyer email"),
				31:  translator.T(lang, "Buyer phone"),
				32:  translator.T(lang, "Buyer website"),
				33:  translator.T(lang, "Buyer physical person"),
				34:  translator.T(lang, "Buyer banks"),
				100: translator.T(lang, "Seller"),
				101: translator.T(lang, "Buyer"),
			}
			if listURL == "/companies" {
				columnLabels = map[int]string{
					0: translator.T(lang, "Company name"),
					1: translator.T(lang, "Code"),
					2: translator.T(lang, "VAT code"),
					3: translator.T(lang, "City"),
					4: translator.T(lang, "Country"),
					5: translator.T(lang, "Purchases"),
					6: translator.T(lang, "Sales"),
				}
			}
			var out []map[string]any
			// Use a stable order for columns, including special ones
			cols := make([]int, 0, len(filters))
			for c := range filters {
				cols = append(cols, c)
			}
			// Sort cols to keep UI stable
			for i := 0; i < len(cols); i++ {
				for j := i + 1; j < len(cols); j++ {
					if cols[i] > cols[j] {
						cols[i], cols[j] = cols[j], cols[i]
					}
				}
			}

			for _, col := range cols {
				vals, ok := filters[col]
				if !ok {
					continue
				}
				label := columnLabels[col]
				if label == "" {
					continue // Skip unknown columns
				}
				for _, v := range vals {
					if strings.TrimSpace(v) != "" {
						displayValue := v
						if listURL == "/invoices" && col == 16 {
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
			columnLabels := map[int]string{
				0:   translator.T(lang, "Created"),
				1:   translator.T(lang, "Number"),
				2:   translator.T(lang, "Type"),
				3:   translator.T(lang, "Date"),
				4:   translator.T(lang, "Service date"),
				5:   translator.T(lang, "Payment date"),
				6:   translator.T(lang, "Seller"),
				7:   translator.T(lang, "Seller code"),
				8:   translator.T(lang, "Seller VAT"),
				9:   translator.T(lang, "Buyer"),
				10:  translator.T(lang, "Buyer code"),
				11:  translator.T(lang, "Buyer VAT"),
				12:  translator.T(lang, "Amount excl. VAT"),
				13:  translator.T(lang, "VAT"),
				14:  translator.T(lang, "Amount incl. VAT"),
				15:  translator.T(lang, "Currency"),
				16:  translator.T(lang, "Status"),
				17:  translator.T(lang, "Seller street"),
				18:  translator.T(lang, "Seller city"),
				19:  translator.T(lang, "Seller country"),
				20:  translator.T(lang, "Seller postal code"),
				21:  translator.T(lang, "Seller email"),
				22:  translator.T(lang, "Seller phone"),
				23:  translator.T(lang, "Seller website"),
				24:  translator.T(lang, "Seller physical person"),
				25:  translator.T(lang, "Seller banks"),
				26:  translator.T(lang, "Buyer street"),
				27:  translator.T(lang, "Buyer city"),
				28:  translator.T(lang, "Buyer country"),
				29:  translator.T(lang, "Buyer postal code"),
				30:  translator.T(lang, "Buyer email"),
				31:  translator.T(lang, "Buyer phone"),
				32:  translator.T(lang, "Buyer website"),
				33:  translator.T(lang, "Buyer physical person"),
				34:  translator.T(lang, "Buyer banks"),
				100: translator.T(lang, "Seller"),
				101: translator.T(lang, "Buyer"),
			}
			if listURL == "/companies" {
				columnLabels = map[int]string{
					0: translator.T(lang, "Company name"),
					1: translator.T(lang, "Code"),
					2: translator.T(lang, "VAT code"),
					3: translator.T(lang, "City"),
					4: translator.T(lang, "Country"),
					5: translator.T(lang, "Purchases"),
					6: translator.T(lang, "Sales"),
				}
			}
			return columnLabels[col]
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
	})

	tmpl, err = tmpl.ParseFS(TemplateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{tmpl: tmpl, translator: translator}, nil
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
	if c, err := req.Cookie("csrf_token"); err == nil {
		m["CSRFToken"] = c.Value
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
	if c, err := req.Cookie("csrf_token"); err == nil {
		m["CSRFToken"] = c.Value
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
