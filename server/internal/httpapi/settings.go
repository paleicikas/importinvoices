package httpapi

import (
	"context"
	"net/http"

	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/reqctx"
	"github.com/paleicikas/importinvoices/server/internal/service"
)

func userFromContext(ctx context.Context) (*domain.User, bool) {
	return reqctx.User(ctx)
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		for key, val := range map[string]string{
			"llm_provider":   r.FormValue("llm_provider"),
			"openai_api_key": r.FormValue("openai_api_key"),
			"openai_model":   r.FormValue("openai_model"),
			"google_api_key": r.FormValue("google_api_key"),
			"google_model":   r.FormValue("google_model"),
			"mcp_token":      r.FormValue("mcp_token"),
		} {
			if err := s.svc.SetSetting(ctx, key, val); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Organization settings
		if r.Form.Has("org_title") {
			if err := s.svc.UpdateOrganization(ctx, r.FormValue("org_title")); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		
		s.setFlash(w, "Settings saved successfully", "success")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	settings, err := s.svc.GetAllSettings(r.Context())
	if err != nil {
		s.setFlash(w, err.Error(), "error")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	org, _ := s.svc.GetOrganization(r.Context())

	s.render.RenderPage(w, r, "settings.html", map[string]any{
		"Title":        "Settings",
		"Page":         "settings",
		"Settings":     settings,
		"Organization": org,
	})
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromContext(r.Context())

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if r.Form.Has("password") && r.FormValue("password") != "" {
			if r.FormValue("password") != r.FormValue("password_repeat") {
				s.setFlash(w, "Passwords do not match", "error")
				http.Redirect(w, r, "/profile", http.StatusSeeOther)
				return
			}
			if err := s.svc.UpdatePassword(r.Context(), user.ID, r.FormValue("password")); err != nil {
				s.setFlash(w, err.Error(), "error")
				http.Redirect(w, r, "/profile", http.StatusSeeOther)
				return
			}
			session, err := s.svc.CreateSession(r.Context(), user.ID)
			if err != nil {
				s.setFlash(w, err.Error(), "error")
				http.Redirect(w, r, "/profile", http.StatusSeeOther)
				return
			}
			http.SetCookie(w, newSessionCookie(r, session.Token, session.ExpiresAt))
		}

		if err := s.svc.UpdateUser(r.Context(), user.ID, r.FormValue("name"), r.FormValue("email")); err != nil {
			s.setFlash(w, err.Error(), "error")
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		webhooks := map[string]string{
			"invoice.confirmed": r.FormValue("webhook_confirmed"),
			"invoice.exported":  r.FormValue("webhook_exported"),
			"invoice.processed":   r.FormValue("webhook_processed"),
		}
		if err := s.svc.UpdateUserWebhooks(r.Context(), user.ID, webhooks); err != nil {
			s.setFlash(w, err.Error(), "error")
			http.Redirect(w, r, "/profile", http.StatusSeeOther)
			return
		}

		s.setFlash(w, "Profile updated successfully", "success")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	s.render.RenderPage(w, r, "profile.html", map[string]any{
		"Title":             "Your Profile",
		"Page":              "profile",
		"WebhookConfirmed":  service.WebhookURLForEvent(user.WebhookUrls, "invoice.confirmed"),
		"WebhookExported":   service.WebhookURLForEvent(user.WebhookUrls, "invoice.exported"),
		"WebhookProcessed":  service.WebhookURLForEvent(user.WebhookUrls, "invoice.processed"),
	})
}
