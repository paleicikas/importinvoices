package service

import (
	"context"
	"testing"

	"github.com/paleicikas/importinvoices/server/internal/domain"
)

func TestVatClassifierCRUD(t *testing.T) {
	svc, _, _, _ := NewTestService(t)

	ctx := context.Background()
	orgID := "test-org"
	_, _ = svc.store.DB().Exec("INSERT INTO organizations (id, title) VALUES (?, ?)", orgID, "Test Org")

	// 1. Create
	vc := &domain.VatClassifier{
		OrgID:   orgID,
		Country: "LT",
		Code:    "PVM1",
		Tariff:  21,
		Active:  true,
	}
	if err := svc.CreateVatClassifier(ctx, vc); err != nil {
		t.Fatalf("failed to create: %v", err)
	}

	// 2. List
	list, err := svc.ListVatClassifiers(ctx, orgID)
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 classifier, got %d", len(list))
	}

	// 3. Update
	vc.Description = strPtr("Standard rate")
	if err := svc.UpdateVatClassifier(ctx, vc); err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	// 4. Get
	updated, err := svc.GetVatClassifier(ctx, vc.ID)
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	if *updated.Description != "Standard rate" {
		t.Fatalf("expected 'Standard rate', got %v", updated.Description)
	}

	// 5. Delete
	if err := svc.DeleteVatClassifier(ctx, vc.ID, orgID); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	list, _ = svc.ListVatClassifiers(ctx, orgID)
	if len(list) != 0 {
		t.Fatalf("expected 0 classifiers, got %d", len(list))
	}
}

func TestImportCatalog(t *testing.T) {
	svc, _, _, _ := NewTestService(t)

	ctx := context.Background()
	orgID := "test-org"
	_, _ = svc.store.DB().Exec("INSERT INTO organizations (id, title) VALUES (?, ?)", orgID, "Test Org")

	// 1. Import all
	if err := svc.ImportCatalogCountry(ctx, orgID, "LT", false); err != nil {
		t.Fatalf("failed to import LT: %v", err)
	}

	list, _ := svc.ListVatClassifiers(ctx, orgID)
	if len(list) < 40 {
		t.Fatalf("expected at least 40 LT classifiers, got %d", len(list))
	}

	// 2. Import missing only (should not add anything)
	countBefore := len(list)
	if err := svc.ImportCatalogCountry(ctx, orgID, "LT", true); err != nil {
		t.Fatalf("failed to import missing LT: %v", err)
	}
	list, _ = svc.ListVatClassifiers(ctx, orgID)
	if len(list) != countBefore {
		t.Fatalf("expected %d classifiers, got %d", countBefore, len(list))
	}
}
