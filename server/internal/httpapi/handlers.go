package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/paleicikas/importinvoices/server/internal/service"
)

func (s *Server) setFlash(w http.ResponseWriter, message, flashType string) {
	http.SetCookie(w, &http.Cookie{
		Name:  "flash",
		Value: message,
		Path:  "/",
	})
	http.SetCookie(w, &http.Cookie{
		Name:  "flash_type",
		Value: flashType,
		Path:  "/",
	})
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	needsSetup, err := s.svc.NeedsSetup(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"needs_setup": needsSetup}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type setupRequest struct {
	OrgTitle      string `json:"org_title"`
	AdminName      string `json:"admin_name"`
	AdminEmail     string `json:"admin_email"`
	AdminPassword  string `json:"admin_password"`
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(w, r) {
		return
	}

	needsSetup, err := s.svc.NeedsSetup(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !needsSetup {
		http.Error(w, "System is already set up", http.StatusForbidden)
		return
	}

	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.svc.Setup(r.Context(), req.OrgTitle, req.AdminName, req.AdminEmail, req.AdminPassword); err != nil {
		if errors.Is(err, service.ErrPasswordTooShort) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(w, r) {
		return
	}

	ip := s.clientIP(r)
	if ok, retryAfter := s.loginLimiter.allow(ip); !ok {
		secs := int(retryAfter.Seconds())
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		http.Error(w, loginRateLimitedMsg, http.StatusTooManyRequests)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := s.svc.Authenticate(r.Context(), req.Email, req.Password)
	if err != nil {
		s.loginLimiter.recordFailure(ip)
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	session, err := s.svc.CreateSession(r.Context(), user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.loginLimiter.reset(ip)

	http.SetCookie(w, newSessionCookie(r, session.Token, session.ExpiresAt))

	s.rotateCSRFCookie(w, r)

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSetupPage(w http.ResponseWriter, r *http.Request) {
	needsSetup, err := s.svc.NeedsSetup(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !needsSetup {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	token := s.ensureCSRFCookie(w, r)
	s.render.RenderStandalonePage(w, r, "setup", map[string]any{
		"CSRFToken": token,
	})
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	token := s.ensureCSRFCookie(w, r)
	s.render.RenderStandalonePage(w, r, "login", map[string]any{
		"CSRFToken": token,
	})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	counts, err := s.svc.GetInvoiceCounts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	recentInvoices, _, err := s.svc.ListInvoices(r.Context(), service.InvoiceListParams{
		Tab:     "all",
		SortCol: 0, // created_at
		SortDir: "desc",
		Limit:   5,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render.RenderPage(w, r, "home.html", map[string]any{
		"Title":          "Home",
		"Page":           "home",
		"Counts":         counts,
		"RecentInvoices": recentInvoices,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		_ = s.svc.DeleteSession(r.Context(), cookie.Value)
	}

	http.SetCookie(w, clearSessionCookie(r))

	http.Redirect(w, r, "/login", http.StatusFound)
}

func (s *Server) handleInvoices(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "all"
	}
	search := r.URL.Query().Get("q")

	// Parse column filters
	columnFilters := make(map[int][]string)
	for k, v := range r.URL.Query() {
		if strings.HasPrefix(k, "f.") {
			var col int
			if _, err := fmt.Sscanf(k, "f.%d", &col); err == nil {
				columnFilters[col] = v
			}
		}
	}

	sortCol := 0
	if v := r.URL.Query().Get("sort"); v != "" {
		sortCol, _ = strconv.Atoi(v)
	}
	sortDir := r.URL.Query().Get("dir")
	if sortDir == "" {
		sortDir = "desc"
	}

	invoices, total, err := s.svc.ListInvoices(r.Context(), service.InvoiceListParams{
		Tab:           tab,
		Search:        search,
		ColumnFilters: columnFilters,
		SortCol:       sortCol,
		SortDir:       sortDir,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	counts, err := s.svc.GetInvoiceCounts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	org, _ := s.svc.GetOrganization(r.Context())
	var exportTemplates []service.ExportTemplate
	if org != nil {
		exportTemplates, _ = s.svc.ListExportTemplates(r.Context(), org.ID)
	}

	configured, _ := s.svc.IsLLMConfigured(r.Context())

	s.render.RenderPage(w, r, "invoices.html", map[string]any{
		"Title":           "Invoices",
		"Page":            "invoices",
		"ListURL":         "/invoices",
		"Tab":             tab,
		"Search":          search,
		"ColumnFilters":   columnFilters,
		"SortCol":         sortCol,
		"SortDir":         sortDir,
		"Invoices":        invoices,
		"Total":           total,
		"Counts":          counts,
		"LLMConfigured":   configured,
		"ExportTemplates": exportTemplates,
	})
}

