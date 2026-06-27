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
		languages:    []string{"en", "lt", "de", "fr", "es", "it", "pl", "ru", "lv", "ee"},
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
		if t.isValidLanguage(lang) {
			return lang
		}
	}

	// 2. Check cookie
	if cookie, err := r.Cookie("lang"); err == nil {
		if t.isValidLanguage(cookie.Value) {
			return cookie.Value
		}
	}

	// 3. Check Accept-Language header
	accept := r.Header.Get("Accept-Language")
	if accept != "" {
		parts := strings.Split(accept, ",")
		for _, part := range parts {
			lang := strings.Split(strings.TrimSpace(part), ";")[0]
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
