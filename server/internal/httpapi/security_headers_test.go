package httpapi

import (
	"strings"
	"testing"
)

// TestT19_SecurityHeaders verifies P2-6.a: baseline security response headers
// are present on responses, and HSTS is emitted only over HTTPS.
func TestT19_SecurityHeaders(t *testing.T) {
	ts, client, _ := newTestServer(t)
	setupAndLogin(t, ts, client)

	resp, err := client.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	h := resp.Header
	for _, want := range []string{"Content-Security-Policy", "X-Frame-Options", "X-Content-Type-Options", "Referrer-Policy"} {
		if h.Get(want) == "" {
			t.Errorf("missing security header %q", want)
		}
	}
	if got := h.Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}
	if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	// Plain HTTP test server -> no HSTS.
	if h.Get("Strict-Transport-Security") != "" {
		t.Error("HSTS should not be set over plain HTTP")
	}

	// CSP must allow the CDN origins the templates actually load assets from
	// (Bootstrap, Font Awesome, flag-icons, Google Fonts) plus Gravatar for
	// user avatars. A regression here silently breaks the admin UI because
	// the browser refuses to apply the CDN stylesheets/scripts.
	csp := h.Get("Content-Security-Policy")
	for _, want := range []string{
		"https://cdn.jsdelivr.net",
		"https://cdnjs.cloudflare.com",
		"https://fonts.googleapis.com",
		"https://fonts.gstatic.com",
		"https://www.gravatar.com",
	} {
		if !contains(csp, want) {
			t.Errorf("CSP missing allowed origin %q: %s", want, csp)
		}
	}
	// flag-icons renders flags as background-image SVGs fetched from
	// cdn.jsdelivr.net, so img-src must allow that origin or the language
	// dropdown shows the language code with an empty gap where the flag
	// should be (regression seen in refs#nerodo veliaveliu report).
	imgSrc := cspDirective(csp, "img-src")
	if !contains(imgSrc, "https://cdn.jsdelivr.net") {
		t.Errorf("CSP img-src missing cdn.jsdelivr.net for flag-icons: %s", imgSrc)
	}
}

// TestContentDisposition verifies P2-6.d: filenames are header-injection safe
// and non-ASCII names use RFC 5987 encoding with an ASCII fallback.
func TestContentDisposition(t *testing.T) {
	cases := []struct {
		name, filename string
		wantContains   []string
	}{
		{"ascii", "invoice.pdf", []string{"filename=\"invoice.pdf\""}},
		{"injection", "a\r\nb.pdf", []string{"filename=\"ab.pdf\""}},
		{"traversal", "../etc/passwd", []string{"filename=\"..etcpasswd\""}},
		{"nonascii", "sąskaita.pdf", []string{"filename*=UTF-8''"}},
	}
	for _, c := range cases {
		got := contentDisposition("inline", c.filename)
		for _, w := range c.wantContains {
			if !contains(got, w) {
				t.Errorf("%s: contentDisposition = %q, want substring %q", c.name, got, w)
			}
		}
		if contains(got, "\r") || contains(got, "\n") {
			t.Errorf("%s: contentDisposition contains CR/LF: %q", c.name, got)
		}
	}
	if got := contentDisposition("attachment", "export.json"); !contains(got, "attachment; filename=\"export.json\"") {
		t.Errorf("attachment: %q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// cspDirective extracts the value of a single CSP directive (e.g. "img-src")
// from a serialized CSP header. Returns "" if the directive is absent.
func cspDirective(csp, name string) string {
	for _, part := range strings.Split(csp, ";") {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) > 0 && fields[0] == name {
			return strings.Join(fields[1:], " ")
		}
	}
	return ""
}
