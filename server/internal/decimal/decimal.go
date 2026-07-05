// Package decimal parses numeric strings that may use either Anglo-Saxon or
// European locale formatting, as found on invoices and in manual entry forms.
package decimal

import (
	"strconv"
	"strings"
	"unicode"
)

// Parse interprets s as a decimal number, tolerating common locale formats:
//   - "1234.56", "1234,56"
//   - "1.234,56" (DE/IT/ES/FR: thousands '.', decimal ',')
//   - "1,234.56" (EN/US: thousands ',', decimal '.')
//   - "1 234,56", "1'234.56" (space / apostrophe thousands)
//
// When a single separator is present with a 3-digit trailing group it is
// treated as thousands grouping (e.g. "1.234" -> 1234, "1,234" -> 1234); a
// 1-2 digit trailing group is treated as the decimal separator. This is the
// safer interpretation for currency and tariff fields, which are written with
// at most two fractional digits.
func Parse(s string) (float64, error) {
	t := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '\'' {
			return -1
		}
		return r
	}, s)
	t = strings.TrimSpace(t)
	if t == "" {
		return 0, strconv.ErrSyntax
	}

	hasComma := strings.ContainsRune(t, ',')
	hasDot := strings.ContainsRune(t, '.')

	switch {
	case hasComma && hasDot:
		if strings.LastIndexByte(t, ',') > strings.LastIndexByte(t, '.') {
			t = strings.ReplaceAll(t, ".", "")
			t = strings.ReplaceAll(t, ",", ".")
		} else {
			t = strings.ReplaceAll(t, ",", "")
		}
	case hasComma:
		t = normalizeSingleSeparator(t, ',')
	case hasDot:
		t = normalizeSingleSeparator(t, '.')
	}

	return strconv.ParseFloat(t, 64)
}

// normalizeSingleSeparator resolves a string containing only one kind of
// separator. A single occurrence with 1-2 trailing digits (or more than 3)
// is the decimal separator; a single occurrence with exactly 3 trailing
// digits, or multiple occurrences, is thousands grouping and is dropped.
func normalizeSingleSeparator(s string, sep byte) string {
	if strings.Count(s, string(sep)) == 1 {
		idx := strings.IndexByte(s, sep)
		trailing := len(s) - idx - 1
		if trailing == 3 {
			return s[:idx] + s[idx+1:]
		}
		return s[:idx] + "." + s[idx+1:]
	}
	return strings.ReplaceAll(s, string(sep), "")
}
