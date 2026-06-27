package export

import (
	"encoding/json"
	"html"
	"strconv"
	"strings"
	"time"
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

func formatFloat(v float64, decimals int) string {
	if decimals < 0 {
		decimals = 2
	}
	return strconv.FormatFloat(v, 'f', decimals, 64)
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
