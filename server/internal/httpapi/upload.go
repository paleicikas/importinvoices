package httpapi

import (
	"fmt"
	"net/http"
)

func (s *Server) handleUploadPage(w http.ResponseWriter, r *http.Request) {
	configured, _ := s.svc.IsLLMConfigured(r.Context())

	s.render.RenderPage(w, r, "upload.html", map[string]any{
		"Title":         "Upload Invoices",
		"Page":          "upload",
		"LLMConfigured": configured,
	})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	configured, _ := s.svc.IsLLMConfigured(r.Context())
	if !configured {
		s.setFlash(w, "LLM is not configured. Please go to Settings and configure an LLM provider.", "error")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	if err := r.ParseMultipartForm(s.maxUploadBytes); err != nil {
		s.setFlash(w, err.Error(), "error")
		http.Redirect(w, r, "/upload", http.StatusSeeOther)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		s.setFlash(w, "No files uploaded", "error")
		http.Redirect(w, r, "/upload", http.StatusSeeOther)
		return
	}

	cookie, _ := r.Cookie("session_token")
	user, err := s.svc.GetUserBySessionToken(r.Context(), cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	org, err := s.svc.GetOrganization(r.Context())
	if err != nil {
		s.setFlash(w, "Organization not found", "error")
		http.Redirect(w, r, "/upload", http.StatusSeeOther)
		return
	}

	uploadedCount := 0
	duplicateCount := 0
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			s.setFlash(w, "Failed to open "+fileHeader.Filename+": "+err.Error(), "error")
			http.Redirect(w, r, "/upload", http.StatusSeeOther)
			return
		}

		inv, err := s.svc.ImportInvoice(r.Context(), user.ID, org.ID, fileHeader.Filename, file)
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			s.setFlash(w, "Failed to import "+fileHeader.Filename+": "+err.Error(), "error")
			http.Redirect(w, r, "/upload", http.StatusSeeOther)
			return
		}

		if inv.Status == "duplicate" {
			duplicateCount++
		} else {
			uploadedCount++
		}
	}

	msg := fmt.Sprintf("Successfully uploaded %d files", uploadedCount)
	if duplicateCount > 0 {
		msg += fmt.Sprintf(" (%d duplicates found and moved to Duplicates tab)", duplicateCount)
	}
	s.setFlash(w, msg, "success")
	http.Redirect(w, r, "/invoices?tab=processing", http.StatusSeeOther)
}
