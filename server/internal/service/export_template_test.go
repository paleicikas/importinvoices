package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestExportTemplateCRUD(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()
	org, _ := svc.GetOrganization(ctx)
	orgID := org.ID

	// 1. Create
	tmpl := &ExportTemplate{
		ID:    uuid.New().String(),
		OrgID: orgID,
		Type:  "file",
		Title: "Test Template",
	}
	files := []ExportTemplateFile{
		{ID: uuid.New().String(), Filename: "test.json", Content: `{"id":"test"} `},
	}
	if err := svc.CreateExportTemplate(ctx, tmpl, files); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 2. List
	list, err := svc.ListExportTemplates(ctx, orgID, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, t := range list {
		if t.ID == tmpl.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected template in list")
	}

	// 3. Get
	got, gotFiles, err := svc.GetExportTemplate(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != tmpl.Title {
		t.Errorf("Title = %s, want %s", got.Title, tmpl.Title)
	}
	if len(gotFiles) != 1 {
		t.Errorf("got %d files, want 1", len(gotFiles))
	}

	// 4. Update
	got.Title = "Updated Title"
	if err := svc.UpdateExportTemplate(ctx, got, gotFiles); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// 5. Preview
	previews, err := svc.PreviewExportTemplate(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(previews) != 1 {
		t.Errorf("got %d previews, want 1", len(previews))
	}

	// 6. Delete
	if err := svc.DeleteExportTemplate(ctx, tmpl.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestListExportTemplatesLimit(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	_ = SetupUser(t, svc)
	ctx := context.Background()
	org, _ := svc.GetOrganization(ctx)
	orgID := org.ID

	for i := 0; i < 5; i++ {
		tmpl := &ExportTemplate{
			ID:    uuid.New().String(),
			OrgID: orgID,
			Type:  "file",
			Title: "Cap Template " + uuid.New().String(),
		}
		if err := svc.CreateExportTemplate(ctx, tmpl, []ExportTemplateFile{
			{ID: uuid.New().String(), Filename: "test.json", Content: `{"id":"test"}`},
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	all, err := svc.ListExportTemplates(ctx, orgID, 0)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) < 5 {
		t.Fatalf("expected at least 5 templates, got %d", len(all))
	}

	capped, err := svc.ListExportTemplates(ctx, orgID, 3)
	if err != nil {
		t.Fatalf("List capped: %v", err)
	}
	if len(capped) != 3 {
		t.Fatalf("expected 3 templates with limit=3, got %d", len(capped))
	}
}
