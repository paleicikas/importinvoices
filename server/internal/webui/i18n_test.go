package webui

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// TestLocaleKeyParity enforces that every locale file defines exactly the same
// key set as en.json (the canonical source) and has no empty values. Templates
// look up translations by the English string, so a missing key in a locale
// silently falls back to English and produces a mixed-language UI. Extra keys
// not present in en.json are dead cruft (e.g. the old German "KI" variants)
// that templates never look up.
func TestLocaleKeyParity(t *testing.T) {
	entries, err := localesFS.ReadDir("locales")
	if err != nil {
		t.Fatalf("read locales dir: %v", err)
	}

	readKeys := func(name string) (map[string]string, []string) {
		data, err := localesFS.ReadFile("locales/" + name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal %s: %v", name, err)
		}
		keys := make([]string, 0, len(m))
		for k, v := range m {
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s: empty value for key %q", name, k)
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return m, keys
	}

	_, enKeys := readKeys("en.json")
	enSet := make(map[string]struct{}, len(enKeys))
	for _, k := range enKeys {
		enSet[k] = struct{}{}
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if e.Name() == "en.json" {
			continue
		}
		_, keys := readKeys(e.Name())

		// No locale may carry keys that en.json does not have (dead cruft).
		for _, k := range keys {
			if _, ok := enSet[k]; !ok {
				t.Errorf("%s: extra key not in en.json: %q (likely dead cruft)", e.Name(), k)
			}
		}

		// Exact key-set parity with en.json.
		if len(keys) != len(enKeys) {
			t.Errorf("%s: %d keys, want %d (same as en.json)", e.Name(), len(keys), len(enKeys))
		}
		seen := make(map[string]struct{}, len(keys))
		for _, k := range keys {
			seen[k] = struct{}{}
		}
		for _, k := range enKeys {
			if _, ok := seen[k]; !ok {
				t.Errorf("%s: missing key present in en.json: %q", e.Name(), k)
			}
		}
	}
}
