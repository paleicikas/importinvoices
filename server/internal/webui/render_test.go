package webui

import (
	"html/template"
	"net/url"
	"strings"
	"testing"
)

func newTestRenderer(t *testing.T) *Renderer {
	t.Helper()
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	return r
}

// fn retrieves a typed FuncMap entry, failing the test if the cast does not
// match so a future signature change is caught loudly instead of panicking.
func fn[T any](t *testing.T, r *Renderer, name string) T {
	t.Helper()
	v, ok := r.funcs[name]
	if !ok {
		t.Fatalf("FuncMap has no entry %q", name)
	}
	f, ok := v.(T)
	if !ok {
		t.Fatalf("FuncMap %q = %T, want %T", name, v, *new(T))
	}
	return f
}

func TestFuncMap_ListFilterURL(t *testing.T) {
	r := newTestRenderer(t)
	f := fn[func(string, int, string, map[int][]string, string, int, string, string) string](t, r, "listFilterURL")
	filters := map[int][]string{16: {"exported"}, 6: {"Acme"}}

	got := f("/invoices", 3, "2026-01-01", filters, "foo", 16, "desc", "all")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse %q: %v", got, err)
	}
	q := u.Query()
	if got, want := u.Path, "/invoices"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	if got, want := q.Get("tab"), "all"; got != want {
		t.Errorf("tab = %q, want %q", got, want)
	}
	if got, want := q.Get("q"), "foo"; got != want {
		t.Errorf("q = %q, want %q", got, want)
	}
	if got, want := q.Get("sort"), "16"; got != want {
		t.Errorf("sort = %q, want %q", got, want)
	}
	if got, want := q.Get("dir"), "desc"; got != want {
		t.Errorf("dir = %q, want %q", got, want)
	}
	if got, want := q["f.3"], []string{"2026-01-01"}; len(got) != 1 || got[0] != want[0] {
		t.Errorf("f.3 = %v, want %v", got, want)
	}
	if got, want := q.Get("f.16"), "exported"; got != want {
		t.Errorf("f.16 = %q, want %q (preserved)", got, want)
	}
	if got, want := q.Get("f.6"), "Acme"; got != want {
		t.Errorf("f.6 = %q, want %q (preserved)", got, want)
	}
}

func TestFuncMap_ListFilterURL_ReplacesSameColumn(t *testing.T) {
	r := newTestRenderer(t)
	f := fn[func(string, int, string, map[int][]string, string, int, string, string) string](t, r, "listFilterURL")
	filters := map[int][]string{16: {"exported"}}

	got := f("/invoices", 16, "failed", filters, "", 0, "", "")
	u, _ := url.Parse(got)
	if got, want := len(u.Query()["f.16"]), 1; got != want {
		t.Errorf("f.16 count = %d, want %d (replace not append): %s", got, want, u.String())
	}
	if got, want := u.Query().Get("f.16"), "failed"; got != want {
		t.Errorf("f.16 = %q, want %q", got, want)
	}
}

func TestFuncMap_ListSortURL_Toggle(t *testing.T) {
	r := newTestRenderer(t)
	f := fn[func(string, int, map[int][]string, string, int, string, string) string](t, r, "listSortURL")
	filters := map[int][]string{6: {"Acme"}}

	// Sorting by col 3 when currently sorted by col 16 -> asc.
	got := f("/invoices", 3, filters, "", 16, "desc", "all")
	u, _ := url.Parse(got)
	if got, want := u.Query().Get("sort"), "3"; got != want {
		t.Errorf("sort = %q, want %q", got, want)
	}
	if got, want := u.Query().Get("dir"), "asc"; got != want {
		t.Errorf("dir = %q, want %q (first click asc)", got, want)
	}
	if got, want := u.Query().Get("f.6"), "Acme"; got != want {
		t.Errorf("f.6 = %q, want %q (preserved)", got, want)
	}

	// Sorting by col 3 when already asc on col 3 -> desc.
	got = f("/invoices", 3, filters, "", 3, "asc", "all")
	u, _ = url.Parse(got)
	if got, want := u.Query().Get("dir"), "desc"; got != want {
		t.Errorf("dir = %q, want %q (toggle from asc)", got, want)
	}
}

func TestFuncMap_ListResetURL(t *testing.T) {
	r := newTestRenderer(t)
	f := fn[func(string) string](t, r, "listResetURL")
	if got, want := f("/invoices"), "/invoices"; got != want {
		t.Errorf("listResetURL = %q, want %q", got, want)
	}
}

func TestFuncMap_ListFilterRemoveURL(t *testing.T) {
	r := newTestRenderer(t)
	f := fn[func(string, int, string, map[int][]string, string, int, string, string) string](t, r, "listFilterRemoveURL")
	filters := map[int][]string{16: {"exported", "failed"}, 6: {"Acme"}}

	got := f("/invoices", 16, "exported", filters, "", 0, "", "all")
	u, _ := url.Parse(got)
	if got, want := len(u.Query()["f.16"]), 1; got != want {
		t.Errorf("f.16 count = %d, want %d: %s", got, want, u.String())
	}
	if got, want := u.Query().Get("f.16"), "failed"; got != want {
		t.Errorf("f.16 = %q, want %q (remaining value)", got, want)
	}
	if got, want := u.Query().Get("f.6"), "Acme"; got != want {
		t.Errorf("f.6 = %q, want %q (preserved)", got, want)
	}
}

func TestFuncMap_LangURL(t *testing.T) {
	r := newTestRenderer(t)
	f := fn[func(*url.URL, string) string](t, r, "langURL")

	// With a real *url.URL: existing query preserved, lang set.
	in, _ := url.Parse("/invoices?tab=all&q=foo")
	got := f(in, "de")
	parsed, _ := url.Parse(got)
	if got, want := parsed.Query().Get("lang"), "de"; got != want {
		t.Errorf("lang = %q, want %q", got, want)
	}
	if got, want := parsed.Query().Get("q"), "foo"; got != want {
		t.Errorf("q lost = %q, want %q", got, want)
	}

	// Nil URL -> just ?lang=de.
	if got, want := f(nil, "de"), "?lang=de"; got != want {
		t.Errorf("langURL(nil) = %q, want %q", got, want)
	}
}

func TestFuncMap_ColFilterHelpers(t *testing.T) {
	r := newTestRenderer(t)
	value := fn[func(map[int][]string, int) string](t, r, "colFilterValue")
	at := fn[func(map[int][]string, int, int) string](t, r, "colFilterValueAt")
	active := fn[func(map[int][]string, int) bool](t, r, "colFilterActive")

	filters := map[int][]string{16: {"exported", "failed"}, 3: {"", "  "}}
	if got, want := value(filters, 16), "exported"; got != want {
		t.Errorf("colFilterValue = %q, want %q", got, want)
	}
	if got, want := at(filters, 16, 1), "failed"; got != want {
		t.Errorf("colFilterValueAt = %q, want %q", got, want)
	}
	if got, want := at(filters, 16, 9), ""; got != want {
		t.Errorf("colFilterValueAt out-of-range = %q, want %q", got, want)
	}
	if got, want := active(filters, 16), true; got != want {
		t.Errorf("colFilterActive 16 = %v, want %v", got, want)
	}
	if got, want := active(filters, 3), false; got != want {
		t.Errorf("colFilterActive 3 = %v, want %v (only blanks)", got, want)
	}
	if got, want := active(nil, 16), false; got != want {
		t.Errorf("colFilterActive nil = %v, want %v", got, want)
	}
}

func TestFuncMap_HasActiveFilters(t *testing.T) {
	r := newTestRenderer(t)
	f := fn[func(map[int][]string) bool](t, r, "hasActiveFilters")
	if got, want := f(map[int][]string{3: {"", "  "}}), false; got != want {
		t.Errorf("hasActiveFilters (only blanks) = %v, want %v", got, want)
	}
	if got, want := f(map[int][]string{3: {"x"}}), true; got != want {
		t.Errorf("hasActiveFilters (has value) = %v, want %v", got, want)
	}
	if got, want := f(nil), false; got != want {
		t.Errorf("hasActiveFilters nil = %v, want %v", got, want)
	}
}

func TestFuncMap_ArithmeticAndSeq(t *testing.T) {
	r := newTestRenderer(t)
	add := fn[func(int, int) int](t, r, "add")
	sub := fn[func(int, int) int](t, r, "sub")
	seq := fn[func(int, int) []int](t, r, "seq")
	ne := fn[func(any, any) bool](t, r, "ne")

	if got, want := add(2, 3), 5; got != want {
		t.Errorf("add = %d, want %d", got, want)
	}
	if got, want := sub(10, 4), 6; got != want {
		t.Errorf("sub = %d, want %d", got, want)
	}
	if got, want := seq(1, 3), []int{1, 2, 3}; len(got) != len(want) {
		t.Errorf("seq = %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("seq[%d] = %d, want %d", i, got[i], want[i])
			}
		}
	}
	if got, want := ne(1, 1), false; got != want {
		t.Errorf("ne 1 1 = %v, want %v", got, want)
	}
	if got, want := ne(1, 2), true; got != want {
		t.Errorf("ne 1 2 = %v, want %v", got, want)
	}
}

func TestFuncMap_SliceAndSplit(t *testing.T) {
	r := newTestRenderer(t)
	slice := fn[func(string, int, int) string](t, r, "slice")
	split := fn[func(string, string) []string](t, r, "split")

	if got, want := slice("ABCDEF", 1, 4), "BCD"; got != want {
		t.Errorf("slice = %q, want %q", got, want)
	}
	if got, want := slice("AB", 0, 99), "AB"; got != want {
		t.Errorf("slice past end = %q, want %q", got, want)
	}
	if got, want := split("a,b,c", ","), []string{"a", "b", "c"}; len(got) != len(want) {
		t.Errorf("split len = %d, want %d", len(got), len(want))
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("split[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	}
	if got := split("", ","); got != nil {
		t.Errorf("split empty = %v, want nil", got)
	}
}

func TestFuncMap_StatusLabelAndBadge(t *testing.T) {
	r := newTestRenderer(t)
	label := fn[func(string, string) string](t, r, "statusLabel")
	badge := fn[func(string) string](t, r, "statusBadgeClass")

	if got := label("en", "processed"); !strings.Contains(strings.ToLower(got), "awaiting") {
		t.Errorf("statusLabel processed = %q, want 'Awaiting confirmation'", got)
	}
	if got, want := label("en", "weird"), "weird"; got != want {
		t.Errorf("statusLabel unknown = %q, want passthrough %q", got, want)
	}
	if got, want := badge("failed"), "bg-danger"; got != want {
		t.Errorf("statusBadgeClass failed = %q, want %q", got, want)
	}
	if got, want := badge("unknown"), "bg-info"; got != want {
		t.Errorf("statusBadgeClass unknown = %q, want %q (default)", got, want)
	}
}

func TestFuncMap_DerefAndCents(t *testing.T) {
	r := newTestRenderer(t)
	derefFloat := fn[func(*float64) float64](t, r, "derefFloat")
	derefInt := fn[func(*int) int](t, r, "derefInt")
	derefString := fn[func(*string) string](t, r, "derefString")
	centsToFloat := fn[func(*int64) float64](t, r, "centsToFloat")

	f := 12.5
	if got, want := derefFloat(&f), 12.5; got != want {
		t.Errorf("derefFloat = %v, want %v", got, want)
	}
	if got, want := derefFloat(nil), 0.0; got != want {
		t.Errorf("derefFloat nil = %v, want %v", got, want)
	}
	i := 7
	if got, want := derefInt(&i), 7; got != want {
		t.Errorf("derefInt = %d, want %d", got, want)
	}
	if got, want := derefInt(nil), 0; got != want {
		t.Errorf("derefInt nil = %d, want %d", got, want)
	}
	s := "hello"
	if got, want := derefString(&s), "hello"; got != want {
		t.Errorf("derefString = %q, want %q", got, want)
	}
	// "null" string is treated as empty (defensive against JSON null).
	nullS := "null"
	if got, want := derefString(&nullS), ""; got != want {
		t.Errorf("derefString null = %q, want %q", got, want)
	}
	if got, want := derefString(nil), ""; got != want {
		t.Errorf("derefString nil = %q, want %q", got, want)
	}
	c := int64(12345) // 123.45
	if got, want := centsToFloat(&c), 123.45; got != want {
		t.Errorf("centsToFloat = %v, want %v", got, want)
	}
	if got, want := centsToFloat(nil), 0.0; got != want {
		t.Errorf("centsToFloat nil = %v, want %v", got, want)
	}
}

func TestFuncMap_Gravatar(t *testing.T) {
	r := newTestRenderer(t)
	f := fn[func(string, int) string](t, r, "gravatar")

	got := f(" User@Example.com ", 80)
	if !strings.HasPrefix(got, "https://www.gravatar.com/avatar/") {
		t.Errorf("gravatar = %q, want gravatar URL prefix", got)
	}
	if !strings.Contains(got, "s=80") {
		t.Errorf("gravatar missing size: %q", got)
	}
	// md5 of "user@example.com" (lowercased + trimmed) is deterministic.
	want := "https://www.gravatar.com/avatar/b58996c504c5638798eb6b511e6f49af?s=80&d=mp"
	if got != want {
		t.Errorf("gravatar = %q, want %q", got, want)
	}
}

func TestFuncMap_CountBanks(t *testing.T) {
	r := newTestRenderer(t)
	f := fn[func(*string) int](t, r, "countBanks")

	banks := `[{"iban":"LT1"},{"iban":"LT2"}]`
	if got, want := f(&banks), 2; got != want {
		t.Errorf("countBanks 2 = %d, want %d", got, want)
	}
	empty := ""
	if got, want := f(&empty), 0; got != want {
		t.Errorf("countBanks empty = %d, want %d", got, want)
	}
	nullArr := "[]"
	if got, want := f(&nullArr), 0; got != want {
		t.Errorf("countBanks [] = %d, want %d", got, want)
	}
	if got, want := f(nil), 0; got != want {
		t.Errorf("countBanks nil = %d, want %d", got, want)
	}
}

func TestFuncMap_FlagUpperLower(t *testing.T) {
	r := newTestRenderer(t)
	flag := fn[func(string) template.HTML](t, r, "flag")
	upper := fn[func(string) string](t, r, "upper")
	lower := fn[func(string) string](t, r, "lower")

	got := string(flag("lt"))
	if !strings.Contains(got, `fi fi-lt`) || !strings.Contains(got, `title="LT"`) {
		t.Errorf("flag lt = %q, want fi-lt span with title=LT", got)
	}
	if got, want := flag(""), template.HTML(""); got != want {
		t.Errorf("flag empty = %q, want %q", got, want)
	}
	if got, want := upper("abc"), "ABC"; got != want {
		t.Errorf("upper = %q, want %q", got, want)
	}
	if got, want := lower("ABC"), "abc"; got != want {
		t.Errorf("lower = %q, want %q", got, want)
	}
}

func TestFuncMap_ColName(t *testing.T) {
	r := newTestRenderer(t)
	f := fn[func(string, int, string) string](t, r, "colName")

	if got := f("en", 16, "/invoices"); !strings.Contains(strings.ToLower(got), "status") {
		t.Errorf("colName invoices 16 = %q, want Status", got)
	}
	if got := f("en", 0, "/companies"); !strings.Contains(strings.ToLower(got), "company name") {
		t.Errorf("colName companies 0 = %q, want Company name", got)
	}
	if got, want := f("en", 999, "/invoices"), ""; got != want {
		t.Errorf("colName unknown = %q, want %q", got, want)
	}
}

func TestFuncMap_ActiveFilters(t *testing.T) {
	r := newTestRenderer(t)
	f := fn[func(string, map[int][]string, string) []map[string]any](t, r, "activeFilters")

	// Two active filters on /invoices, plus a blank that must be skipped.
	filters := map[int][]string{16: {"exported", ""}, 6: {"Acme"}}
	items := f("en", filters, "/invoices")
	if len(items) != 2 {
		t.Fatalf("activeFilters returned %d items, want 2 (blank skipped): %+v", len(items), items)
	}
	// Sorted ascending by col: 6 (Seller) first, then 16 (Status).
	if items[0]["Col"] != 6 || items[0]["Label"] != "Seller" || items[0]["Value"] != "Acme" {
		t.Errorf("activeFilters[0] = %+v, want Col=6 Seller=Acme", items[0])
	}
	if items[1]["Col"] != 16 || items[1]["Label"] != "Status" || items[1]["Value"] != "Exported" {
		t.Errorf("activeFilters[1] = %+v, want Col=16 Status=Exported (status label resolved)", items[1])
	}

	// Unknown column is skipped (no label -> no chip).
	if got := f("en", map[int][]string{999: {"x"}}, "/invoices"); len(got) != 0 {
		t.Errorf("activeFilters with unknown col = %+v, want empty", got)
	}
}

func TestFuncMap_T(t *testing.T) {
	r := newTestRenderer(t)
	tr := fn[func(string, string) string](t, r, "T")
	// Known English key returns the value; unknown key falls back to the key.
	if got := tr("en", "Status"); got != "Status" {
		t.Errorf("T en Status = %q, want Status", got)
	}
	if got, want := tr("en", "no.such.key"), "no.such.key"; got != want {
		t.Errorf("T unknown key = %q, want passthrough %q", got, want)
	}
	// Lithuanian locale resolves a translated string.
	if got := tr("lt", "Status"); got == "Status" {
		t.Errorf("T lt Status = %q, expected a Lithuanian translation", got)
	}
}
