package processor

import (
	"context"
	"path/filepath"
	"testing"
)

func TestNewGeminiProcessorDefaultModel(t *testing.T) {
	p, err := NewGeminiProcessor("test-key", "")
	if err != nil {
		t.Fatalf("NewGeminiProcessor: %v", err)
	}
	if p.model != defaultGeminiModel {
		t.Fatalf("model = %q, want %q", p.model, defaultGeminiModel)
	}
}

func TestGeminiProcessMissingImage(t *testing.T) {
	p, err := NewGeminiProcessor("test-key", defaultGeminiModel)
	if err != nil {
		t.Fatalf("NewGeminiProcessor: %v", err)
	}
	_, err = p.Process(context.Background(), []string{filepath.Join(t.TempDir(), "missing.jpg")}, nil)
	if err == nil {
		t.Fatal("expected error for missing image file")
	}
}

func TestGeminiInvoiceSchema(t *testing.T) {
	schema := geminiInvoiceSchema()
	if schema == nil || schema.Properties == nil {
		t.Fatal("expected schema properties")
	}
	if schema.Properties["items"] == nil {
		t.Fatal("expected items in schema")
	}
	if len(schema.Required) == 0 {
		t.Fatal("expected required fields")
	}
}
