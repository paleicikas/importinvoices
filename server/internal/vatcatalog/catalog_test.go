package vatcatalog

import (
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
