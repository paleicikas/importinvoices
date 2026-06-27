package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/domain"
	"github.com/paleicikas/importinvoices/server/internal/processor"
	"github.com/paleicikas/importinvoices/server/internal/storage"
)

type stubProcessor struct{}

func (stubProcessor) Process(context.Context, []string, []domain.VatClassifier) (*processor.Result, error) {
	return nil, nil
}

func TestGetProcessorUsesOverride(t *testing.T) {
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	strg, err := storage.New(filepath.Join(dir, "files"))
	if err != nil {
		t.Fatal(err)
	}

	svc := New(store, strg, nil)
	svc.SetProcessorOverride(stubProcessor{})

	p, err := svc.GetProcessor(context.Background())
	if err != nil {
		t.Fatalf("GetProcessor: %v", err)
	}
	if _, ok := p.(stubProcessor); !ok {
		t.Fatalf("expected stub processor, got %T", p)
	}
}

func TestIsLLMConfiguredFromEnv(t *testing.T) {
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	strg, err := storage.New(filepath.Join(dir, "files"))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("OPENAI_API_KEY", "env-key")
	svc := New(store, strg, nil)

	ok, err := svc.IsLLMConfigured(context.Background())
	if err != nil {
		t.Fatalf("IsLLMConfigured: %v", err)
	}
	if !ok {
		t.Fatal("expected LLM to be configured from env")
	}
}
