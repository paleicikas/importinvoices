package httpapi

import "net/http"

// securityHeadersMiddleware sets baseline browser security response headers
// (P2-6.a). HSTS is only emitted over HTTPS so local HTTP dev isn't pinned.
//
// The UI (layout.html / setup.html / login.html) loads Bootstrap, Font Awesome,
// flag-icons and Google Fonts from CDNs and renders user avatars from Gravatar,
// so CSP must explicitly allow those origins. flag-icons renders the flag
// glyphs as background-image SVGs fetched from cdn.jsdelivr.net, so that origin
// must also be allowed by img-src (not just style-src) or the language dropdown
// shows the language code with an empty gap where the flag should be.
// Inline styles/scripts are still permitted ('unsafe-inline') because the
// templates use inline <style> blocks and inline <script> handlers, while
// frame embedding, foreign origins beyond the allowlist, and plugin content
// remain forbidden.
func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
			"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://cdnjs.cloudflare.com https://fonts.googleapis.com; " +
			"img-src 'self' data: blob: https://www.gravatar.com https://cdn.jsdelivr.net; " +
			"font-src 'self' data: https://cdnjs.cloudflare.com https://fonts.gstatic.com; " +
			"connect-src 'self'; " +
			"object-src 'none'; " +
			"base-uri 'self'; " +
			"frame-ancestors 'none'"
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		if isSecureRequest(r) {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}
