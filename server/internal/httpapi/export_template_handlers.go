package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/paleicikas/importinvoices/server/internal/service"
)

func (s *Server) handleExportTemplatesPage(w http.ResponseWriter, r *http.Request) {
	org, err := s.svc.GetOrganization(r.Context())
	if err != nil || org == nil {
		http.Error(w, "organization not found", http.StatusBadRequest)
		return
	}

	templates, err := s.svc.ListExportTemplates(r.Context(), org.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render.RenderPage(w, r, "export_templates.html", map[string]any{
		"Title":     "Export Templates",
		"Page":      "settings",
		"ActiveTab": "export-templates",
		"Templates": templates,
	})
}

func (s *Server) handleExportTemplateNewPage(w http.ResponseWriter, r *http.Request) {
	s.render.RenderPage(w, r, "export_template_edit.html", map[string]any{
		"Title":     "New Template",
		"Page":      "settings",
		"ActiveTab": "export-templates",
		"Template":  service.ExportTemplate{Active: true, Type: "file"},
	})
}

func (s *Server) handleExportTemplateCreate(w http.ResponseWriter, r *http.Request) {
	org, err := s.svc.GetOrganization(r.Context())
	if err != nil || org == nil {
		http.Error(w, "organization not found", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tmpl := &service.ExportTemplate{
		ID:          uuid.New().String(),
		OrgID:       org.ID,
		Type:        r.FormValue("type"),
		Title:       r.FormValue("title"),
		Description: stringPtr(r.FormValue("description")),
		Country:     stringPtr(r.FormValue("country")),
		Website:     stringPtr(r.FormValue("website")),
		Active:      r.FormValue("active") == "1",
		IsFavorite:  r.FormValue("is_favorite") == "1",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	var files []service.ExportTemplateFile
	if tmpl.Type == "api" {
		files = append(files, service.ExportTemplateFile{
			ID:         uuid.New().String(),
			TemplateID: tmpl.ID,
			Filename:   "request.yaml",
			Content:    r.FormValue("api_request"),
		})
	} else {
		files = parseFilesFromForm(r, tmpl.ID)
	}

	if err := s.svc.CreateExportTemplate(r.Context(), tmpl, files); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.setFlash(w, "Template created successfully", "success")
	http.Redirect(w, r, "/settings/export-templates", http.StatusFound)
}

func (s *Server) handleExportTemplatePreviewPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tmpl, files, err := s.svc.GetExportTemplate(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	previews, err := s.svc.PreviewExportTemplate(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render.RenderPage(w, r, "export_template_preview.html", map[string]any{
		"Title":     tmpl.Title,
		"Page":      "settings",
		"ActiveTab": "export-templates",
		"Template":  tmpl,
		"Files":     files,
		"Previews":  previews,
	})
}

func (s *Server) handleExportTemplateEditPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tmpl, files, err := s.svc.GetExportTemplate(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	previews := s.svc.PreviewTemplateFiles(files)

	s.render.RenderPage(w, r, "export_template_edit.html", map[string]any{
		"Title":     "Edit Template",
		"Page":      "settings",
		"ActiveTab": "export-templates",
		"Template":  tmpl,
		"Files":     files,
		"Previews":  previews,
	})
}

func (s *Server) handleExportTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	existing, _, err := s.svc.GetExportTemplate(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tmpl := &service.ExportTemplate{
		ID:          id,
		OrgID:       existing.OrgID,
		Type:        existing.Type,
		Title:       r.FormValue("title"),
		Description: stringPtr(r.FormValue("description")),
		Country:     stringPtr(r.FormValue("country")),
		Website:     stringPtr(r.FormValue("website")),
		Active:      r.FormValue("active") == "1",
		IsSystem:    existing.IsSystem,
		IsFavorite:  r.FormValue("is_favorite") == "1",
	}

	var files []service.ExportTemplateFile
	if tmpl.Type == "api" {
		files = append(files, service.ExportTemplateFile{
			ID:         uuid.New().String(),
			TemplateID: tmpl.ID,
			Filename:   "request.yaml",
			Content:    r.FormValue("api_request"),
		})
	} else {
		files = parseFilesFromForm(r, tmpl.ID)
	}

	if err := s.svc.UpdateExportTemplate(r.Context(), tmpl, files); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.setFlash(w, "Template updated successfully", "success")
	http.Redirect(w, r, "/settings/export-templates/"+id, http.StatusFound)
}

func (s *Server) handleExportTemplateFavorite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	tmpl, _, err := s.svc.GetExportTemplate(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	_, err = s.svc.Store().DB().ExecContext(r.Context(),
		"UPDATE export_templates SET is_favorite = ? WHERE id = ?",
		!tmpl.IsFavorite, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/export-templates", http.StatusFound)
}

func (s *Server) handleExportTemplateDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	tmpl, _, err := s.svc.GetExportTemplate(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if tmpl.IsSystem {
		http.Error(w, "cannot delete system template", http.StatusForbidden)
		return
	}

	if err := s.svc.DeleteExportTemplate(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.setFlash(w, "Template deleted successfully", "success")
	http.Redirect(w, r, "/settings/export-templates", http.StatusFound)
}

type templatePreviewRequest struct {
	Type       string `json:"type"`
	APIRequest string `json:"api_request"`
	Files      []struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
	} `json:"files"`
}

func (s *Server) handleExportTemplatePreviewAPI(w http.ResponseWriter, r *http.Request) {
	var req templatePreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var files []service.ExportTemplateFile
	if req.Type == "api" {
		files = append(files, service.ExportTemplateFile{
			Filename: "request.yaml",
			Content:  req.APIRequest,
		})
	} else {
		for _, f := range req.Files {
			if f.Filename == "" {
				continue
			}
			files = append(files, service.ExportTemplateFile{
				Filename: f.Filename,
				Content:  f.Content,
			})
		}
	}

	previews := s.svc.PreviewTemplateFiles(files)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(previews); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func parseFilesFromForm(r *http.Request, templateID string) []service.ExportTemplateFile {
	var files []service.ExportTemplateFile
	for i := 0; ; i++ {
		filename := r.FormValue(fmt.Sprintf("files[%d].filename", i))
		if filename == "" {
			break
		}
		content := r.FormValue(fmt.Sprintf("files[%d].content", i))
		id := r.FormValue(fmt.Sprintf("files[%d].id", i))
		if id == "" {
			id = uuid.New().String()
		}

		files = append(files, service.ExportTemplateFile{
			ID:         id,
			TemplateID: templateID,
			Filename:   filename,
			Content:    content,
		})
	}
	return files
}
