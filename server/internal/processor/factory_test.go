package processor

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestNewOpenAIProvider(t *testing.T) {
	p, err := New("openai", "test-key", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	impl, ok := p.(*OpenAIProcessor)
	if !ok {
		t.Fatalf("expected *OpenAIProcessor, got %T", p)
	}
	if impl.model != openai.GPT4oMini {
		t.Fatalf("model = %q, want %q", impl.model, openai.GPT4oMini)
	}
}

func TestNewGoogleProvider(t *testing.T) {
	p, err := New("google", "test-key", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := p.(*GeminiProcessor); !ok {
		t.Fatalf("expected *GeminiProcessor, got %T", p)
	}
}

func TestNewUnknownProvider(t *testing.T) {
	if _, err := New("anthropic", "test-key", ""); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNewOpenAIUsesCustomModel(t *testing.T) {
	p, err := New("openai", "test-key", "gpt-4o")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	impl := p.(*OpenAIProcessor)
	if impl.model != "gpt-4o" {
		t.Fatalf("model = %q", impl.model)
	}
}
