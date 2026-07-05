package vatcatalog

import (
	"strings"
	"testing"
)

func TestListCountries(t *testing.T) {
	countries, err := ListCountries()
	if err != nil {
		t.Fatalf("ListCountries failed: %v", err)
	}
	if len(countries) < 5 {
		t.Fatalf("expected at least 5 countries, got %d", len(countries))
	}

	foundLT := false
	for _, c := range countries {
		if c == "lt" {
			foundLT = true
			break
		}
	}
	if !foundLT {
		t.Fatalf("LT not found in countries list")
	}
}

func TestGetCatalog(t *testing.T) {
	catalog, err := GetCatalog("LT")
	if err != nil {
		t.Fatalf("GetCatalog(LT) failed: %v", err)
	}
	if catalog.CountryCode != "LT" {
		t.Fatalf("expected LT, got %s", catalog.CountryCode)
	}
	if len(catalog.Entries) < 40 {
		t.Fatalf("expected at least 40 entries for LT, got %d", len(catalog.Entries))
	}

	// Test starter pack
	catalog, err = GetCatalog("DE")
	if err != nil {
		t.Fatalf("GetCatalog(DE) failed: %v", err)
	}
	foundSTD := false
	for _, e := range catalog.Entries {
		if e.Code == "STD" {
			foundSTD = true
			if e.Tariff != 19 {
				t.Fatalf("expected 19%% for DE STD, got %.2f", e.Tariff)
			}
		}
	}
	if !foundSTD {
		t.Fatalf("STD code not found in DE catalog")
	}
}

// TestKnownCodePerCountry asserts each shipped country catalog contains a
// well-known VAT code at its canonical standard (or representative) rate.
// This guards against accidental catalog corruption — a deleted or re-rated
// key entry fails loudly instead of silently breaking VAT classification.
func TestKnownCodePerCountry(t *testing.T) {
	known := []struct {
		country string
		code    string
		tariff  float64
	}{
		{"lt", "PVM1", 21},
		{"de", "STD", 19},
		{"fr", "STD", 20},
		{"es", "STD", 21},
		{"it", "STD", 22},
		{"pl", "STD", 23},
		{"lv", "STD", 21},
		{"ee", "STD", 22},
		{"gb", "STD", 20},
		{"us", "TAX", 0},
	}
	for _, k := range known {
		t.Run(k.country, func(t *testing.T) {
			cat, err := GetCatalog(k.country)
			if err != nil {
				t.Fatalf("GetCatalog(%s): %v", k.country, err)
			}
			if !strings.EqualFold(cat.CountryCode, k.country) {
				t.Errorf("CountryCode = %q, want %q", cat.CountryCode, k.country)
			}
			if len(cat.Entries) == 0 {
				t.Fatalf("%s catalog has no entries", k.country)
			}
			var found *CatalogEntry
			for i := range cat.Entries {
				if cat.Entries[i].Code == k.code {
					found = &cat.Entries[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("%s catalog missing known code %q (have: %v)", k.country, k.code, entryCodes(cat.Entries))
			}
			if found.Tariff != k.tariff {
				t.Errorf("%s %s tariff = %.2f, want %.2f", k.country, k.code, found.Tariff, k.tariff)
			}
		})
	}
}

// TestEveryCountryCatalogLoads is a sanity sweep: every country advertised
// by ListCountries must load and carry at least one entry with a matching
// country code. Catches future-added catalogs that are empty or mislabeled.
func TestEveryCountryCatalogLoads(t *testing.T) {
	countries, err := ListCountries()
	if err != nil {
		t.Fatalf("ListCountries: %v", err)
	}
	if len(countries) == 0 {
		t.Fatal("ListCountries returned no countries")
	}
	for _, c := range countries {
		cat, err := GetCatalog(c)
		if err != nil {
			t.Errorf("GetCatalog(%s): %v", c, err)
			continue
		}
		if !strings.EqualFold(cat.CountryCode, c) {
			t.Errorf("%s: CountryCode = %q, want %q", c, cat.CountryCode, c)
		}
		if len(cat.Entries) == 0 {
			t.Errorf("%s catalog has no entries", c)
		}
	}
}

func entryCodes(entries []CatalogEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Code)
	}
	return out
}
