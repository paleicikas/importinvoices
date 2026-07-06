package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

//go:embed locales/*.json
var localesFS embed.FS

type Translator struct {
	translations map[string]map[string]string
	languages    []string
}

func NewTranslator() (*Translator, error) {
	t := &Translator{
		translations: make(map[string]map[string]string),
		// Derived from the central Locales registry in locales.go so the
		// language list has a single source of truth.
		languages: LanguageCodes(),
	}

	for _, lang := range t.languages {
		data, err := localesFS.ReadFile(fmt.Sprintf("locales/%s.json", lang))
		if err != nil {
			return nil, fmt.Errorf("failed to read locale %s: %w", lang, err)
		}

		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("failed to unmarshal locale %s: %w", lang, err)
		}
		t.translations[lang] = m
	}

	return t, nil
}

func (t *Translator) T(lang, key string) string {
	if m, ok := t.translations[lang]; ok {
		if val, ok := m[key]; ok {
			return val
		}
	}
	// Fallback to English
	if lang != "en" {
		if m, ok := t.translations["en"]; ok {
			if val, ok := m[key]; ok {
				return val
			}
		}
	}
	return key
}

func (t *Translator) GetLanguage(r *http.Request) string {
	// 1. Check query param
	if lang := r.URL.Query().Get("lang"); lang != "" {
		lang = strings.ToLower(lang)
		if t.isValidLanguage(lang) {
			return lang
		}
	}

	// 2. Check cookie
	if cookie, err := r.Cookie("lang"); err == nil {
		lang := strings.ToLower(cookie.Value)
		if t.isValidLanguage(lang) {
			return lang
		}
	}

	// 3. Check Accept-Language header
	accept := r.Header.Get("Accept-Language")
	if accept != "" {
		parts := strings.Split(accept, ",")
		for _, part := range parts {
			lang := strings.Split(strings.TrimSpace(part), ";")[0]
			// Try the full tag first (e.g. "pt-BR", "zh-CN") so region variants
			// match a registered code like "pt-br" before falling back to the
			// 2-char prefix.
			if t.isValidLanguage(strings.ToLower(lang)) {
				return strings.ToLower(lang)
			}
			if len(lang) > 2 {
				lang = lang[:2]
			}
			if t.isValidLanguage(lang) {
				return lang
			}
		}
	}

	return "en"
}

func (t *Translator) isValidLanguage(lang string) bool {
	lang = strings.ToLower(lang)
	for _, l := range t.languages {
		if l == lang {
			return true
		}
	}
	return false
}

func (t *Translator) SetLanguageCookie(w http.ResponseWriter, lang string) {
	if t.isValidLanguage(lang) {
		http.SetCookie(w, &http.Cookie{
			Name:     "lang",
			Value:    lang,
			Path:     "/",
			MaxAge:   365 * 24 * 60 * 60, // 1 year
			HttpOnly: true,
		})
	}
}
