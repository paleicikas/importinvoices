package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	localesDir := "server/internal/webui/locales"
	enPath := filepath.Join(localesDir, "en.json")
	
	enData, err := os.ReadFile(enPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading en.json: %v\n", err)
		os.Exit(1)
	}
	var enMap map[string]string
	if err := json.Unmarshal(enData, &enMap); err != nil {
		fmt.Fprintf(os.Stderr, "Error unmarshaling en.json: %v\n", err)
		os.Exit(1)
	}

	otherLocales := []string{"lt", "de", "fr", "es", "it", "pl", "ru", "lv", "ee"}
	
	for _, lang := range otherLocales {
		path := filepath.Join(localesDir, lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading locale %s: %v\n", lang, err)
			continue
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			fmt.Fprintf(os.Stderr, "Error unmarshaling locale %s: %v\n", lang, err)
			continue
		}

		updated := false
		for k, v := range enMap {
			if _, ok := m[k]; !ok {
				m[k] = v // Fallback to English value
				updated = true
			}
		}

		if updated {
			newData, err := json.MarshalIndent(m, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling locale %s: %v\n", lang, err)
				continue
			}
			if err := os.WriteFile(path, newData, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing locale %s: %v\n", lang, err)
				continue
			}
			fmt.Printf("Updated %s.json with fallback values.\n", lang)
		}
	}
}
