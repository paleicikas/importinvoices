package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleInvoicePreview(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	inv, err := s.svc.GetInvoiceForOrg(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if inv.PreviewPath == nil || *inv.PreviewPath == "" {
		http.NotFound(w, r)
		return
	}
	s.serveStorageFile(w, r, *inv.PreviewPath)
}

func (s *Server) handleInvoiceFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	inv, err := s.svc.GetInvoiceForOrg(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if inv.StoragePath == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", "inline; filename=\""+inv.Filename+"\"")
	s.serveStorageFile(w, r, inv.StoragePath)
}

func (s *Server) serveStorageFile(w http.ResponseWriter, r *http.Request, relPath string) {
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	if relPath == "." || strings.HasPrefix(relPath, "..") || strings.Contains(relPath, "/../") {
		http.NotFound(w, r)
		return
	}

	base, err := filepath.Abs(s.storagePath)
	if err != nil {
		http.Error(w, "storage unavailable", http.StatusInternalServerError)
		return
	}
	fullPath, err := filepath.Abs(filepath.Join(base, filepath.FromSlash(relPath)))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if fullPath != base && !strings.HasPrefix(fullPath, base+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}

	if _, err := os.Stat(fullPath); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, fullPath)
}
