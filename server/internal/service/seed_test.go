package service

import (
	"context"
	"testing"
)

func TestSeedExportTemplates(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	ctx := context.Background()

	// 1. First seed
	if err := svc.SeedExportTemplates(ctx); err != nil {
		t.Fatalf("SeedExportTemplates: %v", err)
	}

	// 2. Idempotency (second seed should not fail)
	if err := svc.SeedExportTemplates(ctx); err != nil {
		t.Fatalf("SeedExportTemplates idempotency: %v", err)
	}

	// 3. Check if system org exists
	var count int
	_ = svc.Store().DB().QueryRow("SELECT COUNT(*) FROM organizations WHERE id = ?", SystemOrgID).Scan(&count)
	if count != 1 {
		t.Errorf("System organization not found")
	}

	// 4. Check if templates seeded
	_ = svc.Store().DB().QueryRow("SELECT COUNT(*) FROM export_templates WHERE is_system = 1").Scan(&count)
	if count == 0 {
		t.Errorf("No system templates seeded")
	}
}
