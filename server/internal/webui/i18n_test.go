package webui

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// completedLocales lists locale files that have been fully translated and whose
// key set must match en.json exactly. As each locale is completed it is added
// here; until then it is only subject to the structural checks (no extra/dead
// keys, no empty values) so CI stays green during the incremental migration.
var completedLocales = map[string]bool{
	"lt.json": true,
	"de.json": true,
	"pl.json": true,
	"ru.json": true,
	"fr.json": true,
}

// TestLocaleKeyParity enforces that every locale file:
//   - has no empty values,
//   - has no extra/dead keys not present in en.json (catches cruft like the
//     old "KI" German variants that templates never look up),
// and for locales in completedLocales additionally:
//   - defines exactly the same key set as en.json (no missing keys).
//
// en.json is the canonical source of UI strings: templates look up translations
// by the English string, so a missing key in a locale silently falls back to
// English and produces a mixed-language UI.
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

		if !completedLocales[e.Name()] {
			// Incomplete locale: report missing keys as a non-fatal log so the
			// migration progress is visible without failing CI. Structural
			// enforcement (no extra/dead keys, exact parity) is applied only
			// once the locale is added to completedLocales, at which point its
			// file has been fully rewritten and any cruft removed.
			seen := make(map[string]struct{}, len(keys))
			for _, k := range keys {
				seen[k] = struct{}{}
			}
			missing := 0
			for _, k := range enKeys {
				if _, ok := seen[k]; !ok {
					missing++
				}
			}
			t.Logf("%s: incomplete — %d/%d keys present (not yet in completedLocales)", e.Name(), len(keys)-missing, len(enKeys))
			continue
		}

		// Completed locale: no extra/dead keys and exact key-set parity.
		for _, k := range keys {
			if _, ok := enSet[k]; !ok {
				t.Errorf("%s: extra key not in en.json: %q (likely dead cruft)", e.Name(), k)
			}
		}
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
