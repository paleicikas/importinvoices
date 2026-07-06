package webui

import "strings"

// Locale is the full metadata for one supported UI language. This file is
// the single source of truth for the language registry — the i18n loader
// (i18n.go), the render helpers (flagClass/isRTL/bcp47/nativeName) and the
// template dropdowns all derive from Locales below. Add a new language here
// and create its locales/<code>.json file; everything else picks it up.
type Locale struct {
	// Code is the file/URL code and the JSON filename without extension,
	// e.g. "en", "ee", "pt-br". Stored lower-case.
	Code string
	// NativeName is the endonym shown in dropdowns, e.g. "Lietuvių",
	// "Português (BR)", "中文".
	NativeName string
	// FlagCode is the ISO-3166 alpha-2 country code used by the flag-icons
	// CSS class, e.g. "gb" for English, "cn" for Chinese, "br" for pt-br.
	// Languages without a one-to-one country mapping pick a representative
	// country (ar→sa, hi→in, he→il, fa→ir).
	FlagCode string
	// BCP47 is the BCP-47 language tag used for <html lang> and JSON-LD
	// inLanguage, e.g. "et" for ee, "zh-Hans" for zh, "pt-BR" for pt-br.
	BCP47 string
	// OGLocale is the Open Graph og:locale value, e.g. "et_EE", "pt_BR".
	OGLocale string
	// Short is the compact uppercase label shown in the navbar toggle
	// (e.g. "EN", "EE", "PT" for pt-br, "ZH" for zh).
	Short string
	// RTL is true for right-to-left scripts (Arabic, Hebrew, Persian).
	RTL bool
}

// Locales is the ordered list of supported languages. English is first and
// acts as the canonical source / fallback language. Order controls dropdown
// display order.
var Locales = []Locale{
	{Code: "en", NativeName: "English", FlagCode: "gb", BCP47: "en", OGLocale: "en_US", Short: "EN"},
	{Code: "lt", NativeName: "Lietuvių", FlagCode: "lt", BCP47: "lt", OGLocale: "lt_LT", Short: "LT"},
	{Code: "de", NativeName: "Deutsch", FlagCode: "de", BCP47: "de", OGLocale: "de_DE", Short: "DE"},
	{Code: "fr", NativeName: "Français", FlagCode: "fr", BCP47: "fr", OGLocale: "fr_FR", Short: "FR"},
	{Code: "es", NativeName: "Español", FlagCode: "es", BCP47: "es", OGLocale: "es_ES", Short: "ES"},
	{Code: "it", NativeName: "Italiano", FlagCode: "it", BCP47: "it", OGLocale: "it_IT", Short: "IT"},
	{Code: "pl", NativeName: "Polski", FlagCode: "pl", BCP47: "pl", OGLocale: "pl_PL", Short: "PL"},
	{Code: "ru", NativeName: "Русский", FlagCode: "ru", BCP47: "ru", OGLocale: "ru_RU", Short: "RU"},
	{Code: "lv", NativeName: "Latviešu", FlagCode: "lv", BCP47: "lv", OGLocale: "lv_LV", Short: "LV"},
	{Code: "ee", NativeName: "Eesti", FlagCode: "ee", BCP47: "et", OGLocale: "et_EE", Short: "EE"},
	{Code: "uk", NativeName: "Українська", FlagCode: "ua", BCP47: "uk", OGLocale: "uk_UA", Short: "UK"},
	{Code: "zh", NativeName: "中文", FlagCode: "cn", BCP47: "zh-Hans", OGLocale: "zh_CN", Short: "ZH"},
	{Code: "ja", NativeName: "日本語", FlagCode: "jp", BCP47: "ja", OGLocale: "ja_JP", Short: "JA"},
	{Code: "ko", NativeName: "한국어", FlagCode: "kr", BCP47: "ko", OGLocale: "ko_KR", Short: "KO"},
	{Code: "ar", NativeName: "العربية", FlagCode: "sa", BCP47: "ar", OGLocale: "ar_SA", Short: "AR", RTL: true},
	{Code: "hi", NativeName: "हिन्दी", FlagCode: "in", BCP47: "hi", OGLocale: "hi_IN", Short: "HI"},
	{Code: "pt-br", NativeName: "Português (BR)", FlagCode: "br", BCP47: "pt-BR", OGLocale: "pt_BR", Short: "PT"},
	{Code: "id", NativeName: "Bahasa Indonesia", FlagCode: "id", BCP47: "id", OGLocale: "id_ID", Short: "ID"},
	{Code: "vi", NativeName: "Tiếng Việt", FlagCode: "vn", BCP47: "vi", OGLocale: "vi_VN", Short: "VI"},
	{Code: "th", NativeName: "ไทย", FlagCode: "th", BCP47: "th", OGLocale: "th_TH", Short: "TH"},
	{Code: "he", NativeName: "עברית", FlagCode: "il", BCP47: "he", OGLocale: "he_IL", Short: "HE", RTL: true},
	{Code: "fa", NativeName: "فارسی", FlagCode: "ir", BCP47: "fa", OGLocale: "fa_IR", Short: "FA", RTL: true},
}

// localeByCode returns the Locale for a code (case-insensitive), or
// ok=false if the code is not registered.
func localeByCode(code string) (Locale, bool) {
	code = strings.ToLower(code)
	for _, l := range Locales {
		if l.Code == code {
			return l, true
		}
	}
	return Locale{}, false
}

// LanguageCodes returns the list of language codes in registry order. This
// is the canonical list consumed by the translator loader.
func LanguageCodes() []string {
	out := make([]string, len(Locales))
	for i, l := range Locales {
		out[i] = l.Code
	}
	return out
}

// IsRTL reports whether the language renders right-to-left.
func IsRTL(lang string) bool {
	if l, ok := localeByCode(lang); ok {
		return l.RTL
	}
	return false
}

// FlagCode returns the ISO-3166 flag code for a language, falling back to
// the language code itself when unknown (so callers always get a usable
// class token).
func FlagCode(lang string) string {
	if l, ok := localeByCode(lang); ok {
		return l.FlagCode
	}
	return lang
}

// BCP47 returns the BCP-47 tag for a language (e.g. "et" for ee, "zh-Hans"
// for zh, "pt-BR" for pt-br), falling back to the code itself.
func BCP47(lang string) string {
	if l, ok := localeByCode(lang); ok {
		return l.BCP47
	}
	return lang
}

// NativeName returns the endonym for a language, falling back to the code.
func NativeName(lang string) string {
	if l, ok := localeByCode(lang); ok {
		return l.NativeName
	}
	return lang
}

// ShortLabel returns the compact uppercase navbar-toggle label for a language,
// falling back to the uppercased code.
func ShortLabel(lang string) string {
	if l, ok := localeByCode(lang); ok {
		return l.Short
	}
	return strings.ToUpper(lang)
}
