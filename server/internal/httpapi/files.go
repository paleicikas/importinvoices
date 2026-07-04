package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// contentDisposition builds a Content-Disposition header value that is safe
// against header injection (no CR/LF/control chars) and encodes non-ASCII
// filenames per RFC 5987 (filename*=UTF-8''...). A sanitized ASCII fallback is
// also provided for legacy clients. `disposition` is "inline" or "attachment".
func contentDisposition(disposition, filename string) string {
	disposition = strings.TrimSpace(disposition)
	if disposition == "" {
		disposition = "inline"
	}
	// Strip control chars and path separators to prevent header injection and
	// path-traversal-style names.
	sanitize := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if r == '\r' || r == '\n' || r == '"' || r == '\\' || r == '/' || r < 0x20 {
				continue
			}
			b.WriteRune(r)
		}
		return b.String()
	}
	ascii := sanitize(strings.ToValidUTF8(filename, "_"))
	if ascii == "" {
		ascii = "file"
	}
	// If the filename is pure ASCII, the simple form is enough.
	if isASCII(ascii) {
		return disposition + "; filename=\"" + ascii + "\""
	}
	// Non-ASCII: provide an ASCII fallback plus the RFC 5987 encoded form.
	return disposition + "; filename=\"" + asciiFallback(ascii) + "\"; filename*=UTF-8''" + encodeRFC5987(filename)
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 0x7F {
			return false
		}
	}
	return true
}

// asciiFallback replaces non-ASCII runes with underscores for the legacy
// filename parameter.
func asciiFallback(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r > 0x7F {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	out := b.String()
	if out == "" {
		return "file"
	}
	return out
}

// encodeRFC5987 percent-encodes a UTF-8 filename for the filename* parameter
// per RFC 5987. attr-chars allow ALPHA/DIGIT/!#$&+-.^_`|~; everything else is
// percent-encoded.
func encodeRFC5987(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			r == '!' || r == '#' || r == '$' || r == '&' || r == '+' || r == '-' ||
			r == '.' || r == '^' || r == '_' || r == '`' || r == '|' || r == '~':
			b.WriteRune(r)
		default:
			for _, c := range []byte(string(r)) {
				const hex = "0123456789ABCDEF"
				b.WriteByte('%')
				b.WriteByte(hex[c>>4])
				b.WriteByte(hex[c&0x0F])
			}
		}
	}
	return b.String()
}

func (s *Server) handleInvoicePreview(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	inv, err := s.svc.GetInvoiceForOrg(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if inv.PreviewPath == nil || *inv.PreviewPath == "" {
		http.NotFound(w, r)
		return
	}
	s.serveStorageFile(w, r, *inv.PreviewPath)
}

func (s *Server) handleInvoiceFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	inv, err := s.svc.GetInvoiceForOrg(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if inv.StoragePath == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", contentDisposition("inline", inv.Filename))
	s.serveStorageFile(w, r, inv.StoragePath)
}

func (s *Server) serveStorageFile(w http.ResponseWriter, r *http.Request, relPath string) {
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	if relPath == "." || strings.HasPrefix(relPath, "..") || strings.Contains(relPath, "/../") {
		http.NotFound(w, r)
		return
	}

	base, err := filepath.Abs(s.storagePath)
	if err != nil {
		http.Error(w, "storage unavailable", http.StatusInternalServerError)
		return
	}
	fullPath, err := filepath.Abs(filepath.Join(base, filepath.FromSlash(relPath)))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if fullPath != base && !strings.HasPrefix(fullPath, base+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}

	if _, err := os.Stat(fullPath); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, fullPath)
}
