package service

import (
	"context"
	"testing"
)

func TestSettings(t *testing.T) {
	svc, _, _, _ := NewTestService(t)
	ctx := context.Background()

	// 1. Set and Get
	err := svc.SetSetting(ctx, "test_key", "test_value")
	if err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	val, err := svc.GetSetting(ctx, "test_key")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if val != "test_value" {
		t.Errorf("got %s, want test_value", val)
	}

	// 2. Get missing
	val, err = svc.GetSetting(ctx, "missing")
	if err != nil {
		t.Fatalf("GetSetting missing: %v", err)
	}
	if val != "" {
		t.Errorf("got %s, want empty", val)
	}

	// 3. Update existing
	err = svc.SetSetting(ctx, "test_key", "new_value")
	if err != nil {
		t.Fatalf("SetSetting update: %v", err)
	}
	val, _ = svc.GetSetting(ctx, "test_key")
	if val != "new_value" {
		t.Errorf("got %s, want new_value", val)
	}

	// 4. GetAll
	all, err := svc.GetAllSettings(ctx)
	if err != nil {
		t.Fatalf("GetAllSettings: %v", err)
	}
	if all["test_key"] != "new_value" {
		t.Errorf("GetAllSettings missing test_key")
	}
}
