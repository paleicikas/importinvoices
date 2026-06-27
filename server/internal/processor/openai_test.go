package processor

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestNewOpenAIProcessorDefaultModel(t *testing.T) {
	p := NewOpenAIProcessor("test-key", "")
	if p.model != openai.GPT4oMini {
		t.Fatalf("model = %q, want %q", p.model, openai.GPT4oMini)
	}
}

func TestOpenAIProcessMissingImage(t *testing.T) {
	p := NewOpenAIProcessor("test-key", "gpt-4o-mini")
	_, err := p.Process(context.Background(), []string{filepath.Join(t.TempDir(), "missing.jpg")}, nil)
	if err == nil {
		t.Fatal("expected error for missing image file")
	}
}

func TestOpenAIGetFunctionParameters(t *testing.T) {
	p := NewOpenAIProcessor("test-key", "gpt-4o-mini")
	params := p.getFunctionParameters()
	if params.Type != "object" {
		t.Fatalf("params type = %q", params.Type)
	}
	if params.Properties["series_and_number"].Type != "string" {
		t.Fatal("expected series_and_number property")
	}
	if params.Properties["items"].Type != "array" {
		t.Fatal("expected items array property")
	}
}
