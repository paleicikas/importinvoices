package httpapi

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/paleicikas/importinvoices/server/internal/reqctx"
	"github.com/paleicikas/importinvoices/server/internal/service"
	"github.com/paleicikas/importinvoices/server/internal/webui"
)

type Server struct {
	svc             *service.Service
	render          *webui.Renderer
	storagePath     string
	maxUploadBytes  int64
	trustedProxies  []string
	loginLimiter    *loginRateLimiter
}

func NewServer(svc *service.Service, render *webui.Renderer, storagePath string, maxUploadBytes int64, trustedProxies []string) *Server {
	return &Server{
		svc:            svc,
		render:         render,
		storagePath:    storagePath,
		maxUploadBytes: maxUploadBytes,
		trustedProxies: trustedProxies,
		loginLimiter:   newLoginRateLimiter(loginRateLimitMax, loginRateLimitWindow),
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(s.securityHeadersMiddleware)

	r.Get("/setup", s.handleSetupPage)
	r.Post("/api/v1/setup", s.handleSetup)
	r.Get("/api/v1/setup/status", s.handleSetupStatus)

	staticFS, _ := fs.Sub(webui.StaticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	r.Get("/login", s.handleLoginPage)
	r.Post("/api/v1/login", s.handleLogin)

	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Use(s.csrfMiddleware)
		r.Post("/logout", s.handleLogout)
		r.Get("/", s.handleIndex)
		r.Get("/invoices", s.handleInvoices)
		r.Get("/invoices/review", s.handleReviewStart)
		r.Get("/companies", s.handleCompanies)
		r.Get("/companies/{id}", s.handleCompanyDetails)
		r.Post("/companies/{id}/delete", s.handleCompanyDelete)
		r.Post("/companies/{id}/merge", s.handleCompanyMerge)
		r.Get("/api/v1/companies/search", s.handleCompanySearchAPI)
		r.Get("/upload", s.handleUploadPage)
		r.Post("/upload", s.handleUpload)
		r.Get("/invoices/{id}/preview", s.handleInvoicePreview)
		r.Get("/invoices/{id}/file", s.handleInvoiceFile)
		r.Get("/invoices/{id}", s.handleReviewPage)
		r.Post("/invoices/{id}", s.handleUpdateInvoice)
		r.Post("/invoices/{id}/confirm", s.handleConfirm)
		r.Post("/invoices/{id}/reprocess", s.handleReprocess)
		r.Post("/export", s.handleExport)
		r.Get("/api/v1/export/templates", s.handleExportTemplatesAPI)
		r.Post("/api/v1/export", s.handleExportAPI)

		// Export templates: list + preview are operator-readable; mutations + edit pages admin-only.
		r.Get("/settings/export-templates", s.handleExportTemplatesPage)
		r.Get("/settings/export-templates/help", s.handleExportTemplateHelpPage)
		r.Get("/settings/export-templates/{id}", s.handleExportTemplatePreviewPage)
		r.Group(func(r chi.Router) {
			r.Use(s.requireAdmin)
			r.Get("/settings/export-templates/new", s.handleExportTemplateNewPage)
			r.Get("/settings/export-templates/{id}/edit", s.handleExportTemplateEditPage)
			r.Post("/settings/export-templates", s.handleExportTemplateCreate)
			r.Post("/settings/export-templates/{id}", s.handleExportTemplateUpdate)
			r.Post("/settings/export-templates/{id}/delete", s.handleExportTemplateDelete)
			r.Post("/settings/export-templates/{id}/favorite", s.handleExportTemplateFavorite)
			r.Post("/api/v1/export/templates/preview", s.handleExportTemplatePreviewAPI)
		})

		r.Get("/profile", s.handleProfile)
		r.Post("/profile", s.handleProfile)

		// VAT classifiers: operator-managed (list, create, edit, delete,
		// import). Export templates remain admin-only for mutations.
		r.Get("/settings/vat-classifiers", s.handleVatClassifiersPage)
		r.Get("/settings/vat-classifiers/new", s.handleVatClassifierNewPage)
		r.Post("/settings/vat-classifiers", s.handleVatClassifierCreate)
		r.Get("/settings/vat-classifiers/{id}/edit", s.handleVatClassifierEditPage)
		r.Post("/settings/vat-classifiers/{id}", s.handleVatClassifierUpdate)
		r.Post("/settings/vat-classifiers/{id}/delete", s.handleVatClassifierDelete)
		r.Post("/settings/vat-classifiers/import", s.handleVatClassifierImport)
		// Settings index is operator-accessible: non-admins are redirected to
		// the first tab they can view (VAT classifiers). The admin-only tabs
		// (LLM, Organization, MCP) and all settings POSTs stay in requireAdmin.
		r.Get("/settings", s.handleSettings)
		r.Group(func(r chi.Router) {
			r.Use(s.requireAdmin)
			r.Get("/settings/llm", s.handleSettings)
			r.Get("/settings/organization", s.handleSettings)
			r.Get("/settings/mcp", s.handleSettings)
			r.Get("/settings/webhooks", s.handleWebhooks)
			r.Post("/settings", s.handleSettings)
			r.Post("/settings/llm", s.handleSettings)
			r.Post("/settings/organization", s.handleSettings)
			r.Post("/settings/mcp", s.handleSettings)
			r.Post("/settings/webhooks", s.handleWebhooks)

			// User management (admin-only).
			r.Get("/settings/users", s.handleUsersPage)
			r.Get("/settings/users/new", s.handleUserNewPage)
			r.Post("/settings/users", s.handleUserCreate)
			r.Get("/settings/users/{id}/edit", s.handleUserEditPage)
			r.Post("/settings/users/{id}", s.handleUserUpdate)
			r.Get("/settings/users/{id}/password", s.handleUserPasswordPage)
			r.Post("/settings/users/{id}/password", s.handleUserPasswordSet)
			r.Post("/settings/users/{id}/delete", s.handleUserDelete)
			r.Post("/settings/users/{id}/role", s.handleUserRoleChange)
		})
	})

	return r
}

// requireAdmin gates a route to admin users. API routes (path prefixed with
// "/api/") and non-GET requests get a 403; browser GET page requests redirect
// to /invoices with an error flash.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reqctx.IsAdmin(r.Context()) {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.Error(w, "admin only", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "admin only", http.StatusForbidden)
			return
		}
		s.setFlash(w, r, "Admin only", "error")
		http.Redirect(w, r, "/invoices", http.StatusSeeOther)
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check if setup is needed
		needsSetup, err := s.svc.NeedsSetup(r.Context())
		if err == nil && needsSetup {
			http.Redirect(w, r, "/setup", http.StatusFound)
			return
		}

		// 2. Check session cookie
		cookie, err := r.Cookie("session_token")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// 3. Verify session
		user, err := s.svc.GetUserBySessionToken(r.Context(), cookie.Value)
		if err != nil {
			// Clear invalid cookie
			http.SetCookie(w, clearSessionCookie(r))
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// 4. Add user and organization to context
		org, _ := s.svc.GetOrganization(r.Context())
		csrfToken := s.ensureCSRFCookie(w, r)
		ctx := reqctx.WithUser(r.Context(), user)
		ctx = reqctx.WithOrganization(ctx, org)
		ctx = reqctx.WithCSRFToken(ctx, csrfToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
