package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	csrfCookieName = "csrf_token"
	csrfFormField  = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
)

func (s *Server) ensureCSRFCookie(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(csrfCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	token := uuid.New().String()
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
	})
	return token
}

func (s *Server) rotateCSRFCookie(w http.ResponseWriter, r *http.Request) {
	token := uuid.New().String()
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
	})
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func newSessionCookie(r *http.Request, token string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Expires:  expires,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	}
}

func clearSessionCookie(r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	}
}

func (s *Server) validateCSRF(w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		http.Error(w, "CSRF token missing", http.StatusForbidden)
		return false
	}

	submitted := r.Header.Get(csrfHeaderName)
	if submitted == "" {
		contentType := r.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "multipart/form-data") {
			if err := r.ParseMultipartForm(s.maxUploadBytes); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return false
			}
			submitted = r.FormValue(csrfFormField)
		} else if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") || contentType == "" {
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return false
			}
			submitted = r.FormValue(csrfFormField)
		}
	}

	if submitted == "" || submitted != cookie.Value {
		http.Error(w, "CSRF token invalid", http.StatusForbidden)
		return false
	}
	return true
}

func (s *Server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if !s.validateCSRF(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}
