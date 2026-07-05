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

func (s *Server) handleUserNewPage(w http.ResponseWriter, r *http.Request) {
	s.render.RenderPage(w, r, "user_edit.html", map[string]any{
		"Title":     "New User",
		"Page":      "settings",
		"ActiveTab": "users",
		"IsNew":     true,
		"Form": map[string]any{
			"Email": "",
			"Name":  "",
			"Role":  string(domain.RoleOperator),
		},
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
		s.render.RenderPage(w, r, "user_edit.html", map[string]any{
			"Title":     "New User",
			"Page":      "settings",
			"ActiveTab": "users",
			"IsNew":     true,
			"Form": map[string]any{
				"Email": email,
				"Name":  name,
				"Role":  role,
			},
		})
		return
	}

	s.setFlash(w, r, "User created successfully", "success")
	http.Redirect(w, r, "/settings/users", http.StatusSeeOther)
}

func (s *Server) handleUserEditPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := s.svc.GetUser(r.Context(), id)
	if err != nil {
		s.setFlash(w, r, err.Error(), "error")
		http.Redirect(w, r, "/settings/users", http.StatusSeeOther)
		return
	}
	s.render.RenderPage(w, r, "user_edit.html", map[string]any{
		"Title":     "Edit User",
		"Page":      "settings",
		"ActiveTab": "users",
		"IsNew":     false,
		"Form": map[string]any{
			"ID":    user.ID,
			"Email": user.Email,
			"Name":  user.Name,
			"Role":  user.Role,
		},
	})
}

func (s *Server) handleUserUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	email := r.FormValue("email")
	role := r.FormValue("role")
	if role == "" {
		role = domain.RoleOperator
	}

	actor, _ := reqctx.User(r.Context())

	current, err := s.svc.GetUser(r.Context(), id)
	if err != nil {
		s.setFlash(w, r, err.Error(), "error")
		http.Redirect(w, r, "/settings/users", http.StatusSeeOther)
		return
	}

	// Role changes go through UpdateUserRole, which enforces last-admin
	// protection and forbids self role-change.
	if current.Role != role {
		if err := s.svc.UpdateUserRole(r.Context(), actor.ID, id, role); err != nil {
			s.setFlash(w, r, err.Error(), "error")
			s.render.RenderPage(w, r, "user_edit.html", map[string]any{
				"Title":     "Edit User",
				"Page":      "settings",
				"ActiveTab": "users",
				"IsNew":     false,
				"Form": map[string]any{
					"ID":    id,
					"Email": email,
					"Name":  name,
					"Role":  current.Role,
				},
			})
			return
		}
	}

	if err := s.svc.UpdateUser(r.Context(), id, name, email); err != nil {
		s.setFlash(w, r, err.Error(), "error")
		s.render.RenderPage(w, r, "user_edit.html", map[string]any{
			"Title":     "Edit User",
			"Page":      "settings",
			"ActiveTab": "users",
			"IsNew":     false,
			"Form": map[string]any{
				"ID":    id,
				"Email": email,
				"Name":  name,
				"Role":  role,
			},
		})
		return
	}

	s.setFlash(w, r, "User updated successfully", "success")
	http.Redirect(w, r, "/settings/users", http.StatusSeeOther)
}

func (s *Server) handleUserPasswordPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := s.svc.GetUser(r.Context(), id)
	if err != nil {
		s.setFlash(w, r, err.Error(), "error")
		http.Redirect(w, r, "/settings/users", http.StatusSeeOther)
		return
	}
	s.render.RenderPage(w, r, "user_password.html", map[string]any{
		"Title":       "Change Password",
		"Page":        "settings",
		"ActiveTab":   "users",
		"TargetUser":  user,
	})
}

func (s *Server) handleUserPasswordSet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	password := r.FormValue("password")
	repeat := r.FormValue("password_repeat")
	if password != repeat {
		s.setFlash(w, r, "Passwords do not match", "error")
		user, _ := s.svc.GetUser(r.Context(), id)
		s.render.RenderPage(w, r, "user_password.html", map[string]any{
			"Title":      "Change Password",
			"Page":       "settings",
			"ActiveTab":  "users",
			"TargetUser": user,
		})
		return
	}

	if err := s.svc.UpdatePassword(r.Context(), id, password); err != nil {
		s.setFlash(w, r, err.Error(), "error")
		user, _ := s.svc.GetUser(r.Context(), id)
		s.render.RenderPage(w, r, "user_password.html", map[string]any{
			"Title":      "Change Password",
			"Page":       "settings",
			"ActiveTab":  "users",
			"TargetUser": user,
		})
		return
	}

	s.setFlash(w, r, "Password updated successfully", "success")
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
