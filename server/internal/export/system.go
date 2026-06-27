package export

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"
)

//go:embed system/*
var systemFS embed.FS

var (
	systemOnce sync.Once
	systemList []TemplateMeta
	systemByID map[string]TemplateMeta
)

func initSystemTemplates() {
	systemByID = make(map[string]TemplateMeta)
	entries, err := fs.ReadDir(systemFS, "system")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "test" {
			continue
		}
		meta, loadErr := loadSystemTemplate(entry.Name())
		if loadErr != nil || !meta.Active {
			continue
		}
		systemList = append(systemList, meta)
		systemByID[meta.ID] = meta
	}
}

type manifest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Country     string   `json:"country"`
	Website     string   `json:"website"`
	Tags        []string `json:"tags"`
	Active      bool     `json:"active"`
	Type        string   `json:"type"`
}

func loadSystemTemplate(dirName string) (TemplateMeta, error) {
	base := path.Join("system", dirName)
	meta := TemplateMeta{
		ID:       "system_" + strings.ToLower(dirName),
		IsSystem: true,
		Active:   true,
		Type:     "file",
		Title:    dirName,
	}

	if data, err := fs.ReadFile(systemFS, path.Join(base, "manifest.json")); err == nil {
		var m manifest
		if json.Unmarshal(data, &m) == nil {
			if m.Title != "" {
				meta.Title = m.Title
			}
			meta.Description = m.Description
			meta.Country = m.Country
			meta.Website = m.Website
			meta.Active = m.Active
			if m.Type != "" {
				meta.Type = m.Type
			}
		}
	}

	files, err := fs.ReadDir(systemFS, base)
	if err != nil {
		return meta, err
	}
	for _, f := range files {
		name := strings.ToLower(f.Name())
		if f.IsDir() || name == "manifest.json" || name == "request.json" || name == "request.yaml" {
			continue
		}
		content, readErr := fs.ReadFile(systemFS, path.Join(base, f.Name()))
		if readErr != nil {
			return meta, readErr
		}
		meta.Files = append(meta.Files, TemplateFile{Filename: f.Name(), Content: string(content)})
	}

	if data, err := fs.ReadFile(systemFS, path.Join(base, "request.json")); err == nil {
		req, parseErr := ParseAPIRequest(string(data))
		if parseErr != nil {
			return meta, parseErr
		}
		meta.Type = "api"
		meta.Request = &req
	}

	return meta, nil
}

func ListSystemTemplates() []TemplateMeta {
	systemOnce.Do(initSystemTemplates)
	out := make([]TemplateMeta, len(systemList))
	copy(out, systemList)
	return out
}

func GetSystemTemplate(id string) (TemplateMeta, bool) {
	systemOnce.Do(initSystemTemplates)
	meta, ok := systemByID[id]
	return meta, ok
}

func SystemTemplateSummary(id string) (TemplateMeta, error) {
	meta, ok := GetSystemTemplate(id)
	if !ok {
		return TemplateMeta{}, fmt.Errorf("system template not found: %s", id)
	}
	summary := meta
	summary.Files = nil
	summary.Request = nil
	return summary, nil
}
