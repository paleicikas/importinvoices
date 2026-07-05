package decimal

import (
	"math"
	"strconv"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		// Anglo-Saxon / plain
		{"1234.56", 1234.56},
		{"1234", 1234},
		{"0.50", 0.50},
		{".5", 0.5},
		{"-21.5", -21.5},
		{"1,234.56", 1234.56}, // EN thousands
		{"1,234", 1234},       // EN thousands, 3-digit group
		{"1,234,567.89", 1234567.89},

		// European
		{"1234,56", 1234.56},
		{"1.234,56", 1234.56},   // DE thousands '.', decimal ','
		{"1.234", 1234},         // DE thousands, 3-digit group
		{"1.234.567,89", 1234567.89},
		{"12,34", 12.34},        // single comma, 2 trailing -> decimal
		{"1,5", 1.5},
		{",5", 0.5},

		// Space / apostrophe thousands
		{"1 234,56", 1234.56},
		{"1'234.56", 1234.56},

		// Whitespace padding
		{"  1234,56  ", 1234.56},

		// More-than-3 trailing digits on single separator -> decimal
		{"1.23456", 1.23456},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q) unexpected err: %v", c.in, err)
			continue
		}
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("Parse(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"", "   ", "abc", "1.2.3,4.5"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) expected error, got nil", in)
		}
	}
}

// TestT16_LocaleParsingDE is the T-16 acceptance test: a German-formatted
// amount string "1.234,56" must parse to the same value as the canonical
// "1234.56". This covers the DE/FR/IT/ES locale case where users or pasted
// invoice data carries thousands '.' and decimal ',' separators.
func TestT16_LocaleParsingDE(t *testing.T) {
	got, err := Parse("1.234,56")
	if err != nil {
		t.Fatalf("Parse(\"1.234,56\") unexpected err: %v", err)
	}
	want := 1234.56
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("Parse(\"1.234,56\") = %v, want %v", got, want)
	}

	// Equivalence with canonical form across a range of realistic amounts.
	for _, c := range []struct{ locale, canonical string }{
		{"1.234,56", "1234.56"},
		{"12.345,00", "12345.00"},
		{"0,99", "0.99"},
		{"1.000.000,50", "1000000.50"},
	} {
		a, err1 := Parse(c.locale)
		b, err2 := Parse(c.canonical)
		if err1 != nil || err2 != nil {
			t.Fatalf("Parse err: %v / %v for %q vs %q", err1, err2, c.locale, c.canonical)
		}
		if math.Abs(a-b) > 1e-9 {
			t.Errorf("Parse(%q)=%v != Parse(%q)=%v", c.locale, a, c.canonical, b)
		}
	}

	// Sanity: empty input returns a strconv error (not a silent zero).
	if _, err := Parse(""); err != strconv.ErrSyntax && err == nil {
		t.Fatalf("Parse(\"\") expected error, got %v", err)
	}
}
