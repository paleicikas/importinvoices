package export

import (
	"encoding/json"
	"html"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

// centsToFloat converts an integer-cent pointer to euros. nil -> 0.
func centsToFloat(v *int64) float64 {
	if v == nil {
		return 0
	}
	return float64(*v) / 100.0
}

func derefBool(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

func derefTime(v *time.Time) time.Time {
	if v == nil {
		return time.Time{}
	}
	return *v
}

func formatDate(t time.Time, layout string) string {
	if t.IsZero() {
		return ""
	}
	if layout == "" {
		layout = "2006-01-02"
	}
	return t.Format(layout)
}

// formatFloat formats a float as a fixed-decimal string using decimal arithmetic
// to avoid binary float rounding drift (e.g. 0.1+0.2 style errors). The result
// always shows exactly `decimals` fractional digits.
func formatFloat(v float64, decimals int) string {
	if decimals < 0 {
		decimals = 2
	}
	d := decimal.NewFromFloat(v).Round(int32(decimals))
	return d.StringFixed(int32(decimals))
}

// formatCents formats integer cents (e.g. 12100) as a fixed-decimal euro string
// with exactly `decimals` fractional digits (e.g. "121.00"). Uses decimal to
// stay exact end-to-end from the integer-cents storage.
func formatCents(cents int64, decimals int) string {
	if decimals < 0 {
		decimals = 2
	}
	d := decimal.New(cents, -2).Round(int32(decimals))
	return d.StringFixed(int32(decimals))
}

func XMLEscape(s string) string {
	return html.EscapeString(s)
}

func CSVEscape(s string) string {
	if s == "" {
		return s
	}
	if strings.ContainsAny(s, ",\"\n\r\t") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

func JSONEscape(s string) string {
	if s == "" {
		return s
	}
	b, err := json.Marshal(s)
	if err != nil {
		return s
	}
	if len(b) >= 2 {
		return string(b[1 : len(b)-1])
	}
	return s
}

func CDATA(s string) string {
	if s == "" {
		return s
	}
	return "<![CDATA[" + strings.ReplaceAll(s, "]]>", "]]]]><![CDATA[>") + "]]>"
}

func companyKey(code, vat, title string) string {
	code = strings.TrimSpace(code)
	vat = strings.TrimSpace(vat)
	title = strings.TrimSpace(strings.ToLower(title))
	if code != "" {
		return "code:" + code
	}
	if vat != "" {
		return "vat:" + vat
	}
	return "title:" + title
}
