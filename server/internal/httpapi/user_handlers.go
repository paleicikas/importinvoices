package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/reqctx"
)

func (s *Server) handleUsersPage(w http.ResponseWriter, r *http.Request) {
	users, err := s.svc.ListUsers(r.Context())
	if err != nil {
		s.setFlash(w, r, err.Error(), "error")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	current, _ := reqctx.User(r.Context())
	s.render.RenderPage(w, r, "users.html", map[string]any{
		"Title":      "Users",
		"Page":       "settings",
		"ActiveTab":  "users",
		"Users":      users,
		"CurrentUser": current,
	})
}

func (s *Server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	name := r.FormValue("name")
	role := r.FormValue("role")
	if role == "" {
		role = domain.RoleOperator
	}

	if _, err := s.svc.CreateUserWithRole(r.Context(), email, password, name, role); err != nil {
		s.setFlash(w, r, err.Error(), "error")
		http.Redirect(w, r, "/settings/users", http.StatusSeeOther)
		return
	}

	s.setFlash(w, r, "User created successfully", "success")
	http.Redirect(w, r, "/settings/users", http.StatusSeeOther)
}

func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	actor, _ := reqctx.User(r.Context())
	if err := s.svc.DeleteUserAs(r.Context(), actor.ID, id); err != nil {
		s.setFlash(w, r, err.Error(), "error")
		http.Redirect(w, r, "/settings/users", http.StatusSeeOther)
		return
	}
	s.setFlash(w, r, "User deleted successfully", "success")
	http.Redirect(w, r, "/settings/users", http.StatusSeeOther)
}

func (s *Server) handleUserRoleChange(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	actor, _ := reqctx.User(r.Context())
	role := r.FormValue("role")
	if err := s.svc.UpdateUserRole(r.Context(), actor.ID, id, role); err != nil {
		s.setFlash(w, r, err.Error(), "error")
		http.Redirect(w, r, "/settings/users", http.StatusSeeOther)
		return
	}
	s.setFlash(w, r, "Role updated successfully", "success")
	http.Redirect(w, r, "/settings/users", http.StatusSeeOther)
}
