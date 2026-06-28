package httpapi

import (
	"io/fs"
	"net/http"

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

	r.Get("/setup", s.handleSetupPage)
	r.Post("/api/v1/setup", s.handleSetup)
	r.Get("/api/v1/setup/status", s.handleSetupStatus)

	staticFS, _ := fs.Sub(webui.StaticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	r.Get("/login", s.handleLoginPage)
	r.Post("/api/v1/login", s.handleLogin)
	r.Get("/logout", s.handleLogout)

	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Use(s.csrfMiddleware)
		r.Get("/", s.handleIndex)
		r.Get("/invoices", s.handleInvoices)
		r.Get("/invoices/review", s.handleReviewStart)
		r.Get("/companies", s.handleCompanies)
		r.Get("/companies/{id}", s.handleCompanyDetails)
		r.Post("/companies/{id}/delete", s.handleCompanyDelete)
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

		r.Get("/settings/export-templates", s.handleExportTemplatesPage)
		r.Get("/settings/export-templates/new", s.handleExportTemplateNewPage)
		r.Post("/settings/export-templates", s.handleExportTemplateCreate)
		r.Get("/settings/export-templates/{id}/edit", s.handleExportTemplateEditPage)
		r.Get("/settings/export-templates/{id}", s.handleExportTemplatePreviewPage)
		r.Post("/settings/export-templates/{id}", s.handleExportTemplateUpdate)
		r.Post("/settings/export-templates/{id}/delete", s.handleExportTemplateDelete)
		r.Post("/settings/export-templates/{id}/favorite", s.handleExportTemplateFavorite)
		r.Post("/api/v1/export/templates/preview", s.handleExportTemplatePreviewAPI)

		r.Get("/profile", s.handleProfile)
		r.Post("/profile", s.handleProfile)
		r.Get("/settings", s.handleSettings)
		r.Get("/settings/llm", s.handleSettings)
		r.Get("/settings/organization", s.handleSettings)
		r.Get("/settings/mcp", s.handleSettings)
		r.Get("/settings/vat-classifiers", s.handleVatClassifiersPage)
		r.Get("/settings/vat-classifiers/new", s.handleVatClassifierNewPage)
		r.Post("/settings/vat-classifiers", s.handleVatClassifierCreate)
		r.Get("/settings/vat-classifiers/{id}/edit", s.handleVatClassifierEditPage)
		r.Post("/settings/vat-classifiers/{id}", s.handleVatClassifierUpdate)
		r.Post("/settings/vat-classifiers/{id}/delete", s.handleVatClassifierDelete)
		r.Post("/settings/vat-classifiers/import", s.handleVatClassifierImport)
		r.Post("/settings", s.handleSettings)
		r.Post("/settings/llm", s.handleSettings)
		r.Post("/settings/organization", s.handleSettings)
		r.Post("/settings/mcp", s.handleSettings)
	})

	return r
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
