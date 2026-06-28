package vatcatalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed data/*.json
var dataFS embed.FS

func ListCountries() ([]string, error) {
	entries, err := dataFS.ReadDir("data")
	if err != nil {
		return nil, err
	}

	var countries []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".json") {
			countries = append(countries, strings.TrimSuffix(name, ".json"))
		}
	}
	return countries, nil
}

func GetCatalog(countryCode string) (*CountryCatalog, error) {
	countryCode = strings.ToLower(countryCode)
	path := "data/" + countryCode + ".json"
	
	data, err := dataFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("country catalog not found: %s", countryCode)
	}

	var catalog CountryCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("failed to unmarshal catalog for %s: %w", countryCode, err)
	}

	return &catalog, nil
}
