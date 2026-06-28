package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/storage"
)

func NewTestDB(t *testing.T) *db.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store
}

func NewTestStorage(t *testing.T) *storage.Storage {
	t.Helper()
	dir := t.TempDir()
	strg, err := storage.New(filepath.Join(dir, "files"))
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	return strg
}

func WriteTestImage(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("server", "internal", "testdata", "sample.png"))
	if err != nil {
		// Fallback if running from different dir
		data, err = os.ReadFile(filepath.Join("..", "..", "testdata", "sample.png"))
		if err != nil {
			t.Fatalf("read sample image: %v", err)
		}
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write test image: %v", err)
	}
}

func WriteTestPDF(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("server", "internal", "testdata", "sample.pdf"))
	if err != nil {
		data, err = os.ReadFile(filepath.Join("..", "..", "testdata", "sample.pdf"))
		if err != nil {
			t.Fatalf("read sample pdf: %v", err)
		}
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write test pdf: %v", err)
	}
}

func MockOpenAIResponse(t *testing.T, resultJSON string) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Mock Chat Completion response with tool call
		response := map[string]any{
			"id":      "chatcmpl-123",
			"object":  "chat.completion",
			"created": 1677652288,
			"model":   "gpt-4o-mini",
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": nil,
						"tool_calls": []any{
							map[string]any{
								"id":   "call_123",
								"type": "function",
								"function": map[string]any{
									"name":      "extract_invoice_data",
									"arguments": resultJSON,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(ts.Close)
	return ts
}

func MockGeminiResponse(t *testing.T, resultJSON string) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Mock Gemini response with function call
		response := map[string]any{
			"candidates": []any{
				map[string]any{
					"content": map[string]any{
						"parts": []any{
							map[string]any{
								"functionCall": map[string]any{
									"name": "extract_invoice_data",
									"args": json.RawMessage(resultJSON),
								},
							},
						},
					},
					"finishReason": "STOP",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(ts.Close)
	return ts
}
