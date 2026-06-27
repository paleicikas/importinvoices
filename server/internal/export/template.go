package export

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"
)

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"xmlEscape":  XMLEscape,
		"csvEscape":  CSVEscape,
		"jsonEscape": JSONEscape,
		"cdata":      CDATA,
		"formatDate": formatDate,
		"formatFloat": formatFloat,
		"isZeroTime": func(t time.Time) bool { return t.IsZero() },
		"companyField": func(c *Company, field string) string {
			if c == nil {
				return ""
			}
			switch field {
			case "title":
				return c.Title
			case "code":
				return c.Code
			case "vat":
				return c.VATIdentificationNumber
			case "street":
				return c.Street
			case "city":
				return c.City
			case "country":
				return c.Country
			case "bank":
				return c.BankAccount
			default:
				return ""
			}
		},
		"last": func(i, n int) bool { return i == n-1 },
		"add":  func(a, b int) int { return a + b },
		"mul":  func(a, b float64) float64 { return a * b },
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"round": func(v float64) int64 {
			if v >= 0 {
				return int64(v + 0.5)
			}
			return int64(v - 0.5)
		},
		"defaultStr": func(fallback, value string) string {
			if strings.TrimSpace(value) == "" {
				return fallback
			}
			return value
		},
		"truncate": func(s string, n int) string {
			if n <= 0 || len(s) <= n {
				return s
			}
			return s[:n]
		},
	}
}

// RenderTemplate executes a Go text/template against an export payload.
func RenderTemplate(name, content string, payload Payload) (string, error) {
	tmpl, err := template.New(name).Funcs(templateFuncs()).Parse(content)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, payload); err != nil {
		return "", fmt.Errorf("render template %q: %w", name, err)
	}
	return strings.TrimSpace(buf.String()), nil
}
