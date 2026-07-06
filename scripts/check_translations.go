package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func main() {
	langFlag := flag.String("lang", "", "Comma-separated list of languages to check (default: all)")
	flag.Parse()

	templatesDir := "internal/webui/templates"
	localesDir := "internal/webui/locales"

	// 1. Extract keys from templates
	keys := make(map[string]bool)
	re := regexp.MustCompile(`\{\{T \$?\.Lang "([^"]+)"\}\}`)

	err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".html" {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			matches := re.FindAllStringSubmatch(string(content), -1)
			for _, m := range matches {
				keys[m[1]] = true
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking templates: %v\n", err)
		os.Exit(1)
	}

	// 2. Determine languages to check
	// Keep in sync with the central Locales registry in
	// server/internal/webui/locales.go.
	allLocales := []string{"en", "lt", "de", "fr", "es", "it", "pl", "ru", "lv", "ee", "uk", "zh", "ja", "ko", "ar", "hi", "pt-br", "id", "vi", "th", "he", "fa"}
	var checkLocales []string
	if *langFlag != "" {
		checkLocales = strings.Split(*langFlag, ",")
	} else {
		checkLocales = allLocales
	}

	// 3. Load locales
	localeData := make(map[string]map[string]string)
	for _, lang := range checkLocales {
		path := filepath.Join(localesDir, strings.TrimSpace(lang)+".json")
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
		localeData[lang] = m
	}

	// 4. Check for missing keys
	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	foundMissing := false
	for _, k := range sortedKeys {
		for _, lang := range checkLocales {
			if _, ok := localeData[lang][k]; !ok {
				if !foundMissing {
					fmt.Println("Missing translations found:")
					foundMissing = true
				}
				fmt.Printf("[%s] Missing key: \"%s\"\n", lang, k)
			}
		}
	}

	if !foundMissing {
		fmt.Printf("All template strings are translated in: %s\n", strings.Join(checkLocales, ", "))
	} else {
		os.Exit(1)
	}
}
