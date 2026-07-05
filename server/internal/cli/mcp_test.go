package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/media"
	"github.com/paleicikas/importinvoices/server/internal/service"
	"github.com/paleicikas/importinvoices/server/internal/storage"
)

// newMCPSvc builds a service with a migrated temp DB and the mcp_token setting
// configured to the given value.
func newMCPSvc(t *testing.T, token string) *service.Service {
	t.Helper()
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	strg, err := storage.New(filepath.Join(dir, "files"))
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(store, strg, media.New(filepath.Join(dir, "temp")))
	if token != "" {
		if err := svc.SetSetting(context.Background(), "mcp_token", token); err != nil {
			t.Fatal(err)
		}
	}
	return svc
}

// runMCPOnce swaps os.Stdin/os.Stdout for pipes, drives runMCPServer with the
// given JSON-RPC requests, and returns the decoded responses.
func runMCPOnce(t *testing.T, svc *service.Service, expectedToken string, requests []JSONRPCRequest) []JSONRPCResponse {
	t.Helper()

	stagingDir := filepath.Join(t.TempDir(), "mcp-imports")
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	rIn, wIn, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = rIn, wOut
	t.Cleanup(func() {
		os.Stdin, os.Stdout = oldIn, oldOut
		_ = rIn.Close()
		_ = wIn.Close()
		_ = rOut.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	done := make(chan struct{})
	go func() {
		_ = runMCPServer(ctx, svc, expectedToken, stagingDir)
		_ = wOut.Close() // signal EOF to the response reader
		close(done)
	}()

	// Write requests then close stdin to let runMCPServer hit EOF and return.
	go func() {
		enc := json.NewEncoder(wIn)
		for _, req := range requests {
			_ = enc.Encode(req)
		}
		_ = wIn.Close()
	}()

	var responses []JSONRPCResponse
	dec := json.NewDecoder(rOut)
	for {
		var res JSONRPCResponse
		if err := dec.Decode(&res); err != nil {
			break
		}
		responses = append(responses, res)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("runMCPServer did not return")
	}
	return responses
}

func TestMCPToolsCallRequiresMatchingPerRequestToken(t *testing.T) {
	svc := newMCPSvc(t, "secret")

	// 1. Wrong per-request token -> -32001 unauthorized.
	wrongReq := JSONRPCRequest{
		JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call",
		Params: json.RawMessage(`{"name":"list_invoices","_meta":{"auth_token":"WRONG"}}`),
	}
	// 2. Correct per-request token -> proceeds (not -32001).
	okReq := JSONRPCRequest{
		JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/call",
		Params: json.RawMessage(`{"name":"list_invoices","_meta":{"auth_token":"secret"}}`),
	}
	// 3. No per-request token -> allowed (session authenticated at startup).
	noneReq := JSONRPCRequest{
		JSONRPC: "2.0", ID: json.RawMessage(`3`), Method: "tools/call",
		Params: json.RawMessage(`{"name":"list_invoices"}`),
	}

	responses := runMCPOnce(t, svc, "secret", []JSONRPCRequest{wrongReq, okReq, noneReq})
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}

	if responses[0].Error == nil || responses[0].Error.Code != -32001 {
		t.Errorf("wrong token: expected error -32001, got %+v", responses[0].Error)
	}
	if responses[1].Error != nil && responses[1].Error.Code == -32001 {
		t.Errorf("correct token: should not be -32001, got %+v", responses[1].Error)
	}
	if responses[2].Error != nil && responses[2].Error.Code == -32001 {
		t.Errorf("no per-request token: should not be -32001 (session auth), got %+v", responses[2].Error)
	}
}

func TestMCPToolsListDoesNotRequireToken(t *testing.T) {
	svc := newMCPSvc(t, "secret")
	req := JSONRPCRequest{
		JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/list",
		Params: json.RawMessage(`{}`),
	}
	responses := runMCPOnce(t, svc, "secret", []JSONRPCRequest{req})
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Error != nil {
		t.Fatalf("tools/list should succeed, got error %+v", responses[0].Error)
	}
	// Result must contain the tool list.
	buf, _ := json.Marshal(responses[0].Result)
	if !strings.Contains(string(buf), "list_invoices") {
		t.Errorf("tools/list result missing list_invoices: %s", string(buf))
	}
}

// TestMCPListInvoicesSchemaNamedKeysOnly verifies the list_invoices `filters`
// schema advertises only named field-name keys (built from
// service.InvoiceColumnIndexByName) and no numeric column ids.
func TestMCPListInvoicesSchemaNamedKeysOnly(t *testing.T) {
	svc := newMCPSvc(t, "secret")
	req := JSONRPCRequest{
		JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/list",
		Params: json.RawMessage(`{}`),
	}
	responses := runMCPOnce(t, svc, "secret", []JSONRPCRequest{req})
	if len(responses) != 1 || responses[0].Error != nil {
		t.Fatalf("tools/list failed: %+v", responses)
	}

	listInvoices, ok := findTool(responses[0].Result, "list_invoices")
	if !ok {
		t.Fatal("list_invoices tool not found in tools/list result")
	}
	schema, _ := listInvoices["inputSchema"].(map[string]any)
	props, _ := schema["properties"].(map[string]any)
	filters, _ := props["filters"].(map[string]any)
	filterProps, _ := filters["properties"].(map[string]any)
	if len(filterProps) == 0 {
		t.Fatal("filters.properties is empty")
	}
	for k := range filterProps {
		if isAllDigits(k) {
			t.Errorf("filters schema advertises numeric column id %q; only named field keys are allowed", k)
		}
	}
	for _, want := range []string{"seller_name", "status", "currency", "vat_codes"} {
		if _, ok := filterProps[want]; !ok {
			t.Errorf("filters schema missing named key %q", want)
		}
	}
}

func findTool(result any, name string) (map[string]any, bool) {
	m, ok := result.(map[string]any)
	if !ok {
		return nil, false
	}
	tools, _ := m["tools"].([]any)
	for _, t := range tools {
		tm, ok := t.(map[string]any)
		if !ok {
			continue
		}
		if tm["name"] == name {
			return tm, true
		}
	}
	return nil, false
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// TestT12_MCPImportInvoicePathTraversal verifies P0-4.c: import_invoice must
// reject absolute paths and traversal that would escape the staging directory.
// /etc/passwd, .. and absolute paths must not be opened.
func TestT12_MCPImportInvoicePathTraversal(t *testing.T) {
	svc := newMCPSvc(t, "secret")

	cases := []struct {
		name string
		path string
	}{
		{"absolute_unix", "/etc/passwd"},
		{"absolute_windows", "C:\\Windows\\system32\\drivers\\etc\\hosts"},
		{"traversal_parent", "../secret.txt"},
		{"traversal_deep", "../../etc/passwd"},
		{"traversal_mixed", "sub/../../secret.txt"},
		{"empty", ""},
	}

	var reqs []JSONRPCRequest
	for i, c := range cases {
		reqs = append(reqs, JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      json.RawMessage(rawJSONNumber(i + 1)),
			Method:  "tools/call",
			Params:  json.RawMessage(`{"name":"import_invoice","arguments":{"path":` + jsonString(c.path) + `}}`),
		})
	}

	responses := runMCPOnce(t, svc, "secret", reqs)
	if len(responses) != len(cases) {
		t.Fatalf("expected %d responses, got %d", len(cases), len(responses))
	}
	for i, c := range cases {
		if responses[i].Error == nil {
			t.Errorf("case %s (%q): expected rejection error, got success", c.name, c.path)
		}
	}
}

// jsonString returns a JSON-quoted string for embedding in raw JSON-RPC params.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// rawJSONNumber returns the literal JSON for a small integer (no surrounding
// quotes), usable as a JSON-RPC id.
func rawJSONNumber(n int) string {
	return strings.TrimSpace(fmt.Sprintf("%d", n))
}

func TestResolveMCPImportPath(t *testing.T) {
	staging := t.TempDir()


	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"relative_simple", "invoice.pdf", false},
		{"relative_subdir", "batch/invoice.pdf", false},
		{"empty", "", true},
		{"absolute_unix", "/etc/passwd", true},
		{"traversal_parent", "../secret.txt", true},
		{"traversal_deep", "../../etc/passwd", true},
		{"traversal_mixed", "sub/../../secret.txt", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveMCPImportPath(staging, c.input)
			if c.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got %q", c.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", c.input, err)
			}
			if !strings.HasPrefix(filepath.ToSlash(got), filepath.ToSlash(staging)+"/") && got != staging {
				t.Errorf("resolved path %q not inside staging %q", got, staging)
			}
		})
	}
}

// TestT13_MCPGetInvoiceCrossOrgRejected verifies P0-4.d: MCP get_invoice must
// reject reads of invoices that belong to a different organization than the
// one the MCP server is scoped to (GetOrganization returns the first org).
func TestT13_MCPGetInvoiceCrossOrgRejected(t *testing.T) {
	svc := newMCPSvc(t, "secret")
	ctx := context.Background()

	// Org A is created first -> becomes the MCP server's org (GetOrganization
	// returns the first non-system org). Org B holds the target invoice.
	orgA, err := svc.CreateOrganization(ctx, "Org A")
	if err != nil {
		t.Fatal(err)
	}
	orgB, err := svc.CreateOrganization(ctx, "Org B")
	if err != nil {
		t.Fatal(err)
	}
	user, err := svc.CreateUser(ctx, "u@b.com", "password1", "User")
	if err != nil {
		t.Fatal(err)
	}

	// Insert an invoice owned by Org B.
	invID := "inv-crossorg-b"
	now := time.Now().Unix()
	_, err = svc.Store().DB().ExecContext(ctx, `
		INSERT INTO invoices (id, user_id, org_id, status, filename, checksum, storage_path, created_at, updated_at)
		VALUES (?, ?, ?, 'processed', 'b.pdf', 'cs-b', 'b/b.pdf', ?, ?)`,
		invID, user.ID, orgB.ID, now, now)
	if err != nil {
		t.Fatal(err)
	}

	// Sanity: MCP server's org must be Org A, not Org B.
	mcpOrg, err := svc.GetOrganization(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if mcpOrg.ID != orgA.ID {
		t.Fatalf("MCP org = %s, want %s (Org A) for a deterministic cross-org test", mcpOrg.ID, orgA.ID)
	}

	req := JSONRPCRequest{
		JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call",
		Params: json.RawMessage(`{"name":"get_invoice","arguments":{"id":"` + invID + `"}}`),
	}
	responses := runMCPOnce(t, svc, "secret", []JSONRPCRequest{req})
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Error == nil {
		b, _ := json.Marshal(responses[0])
		t.Fatalf("expected cross-org get_invoice to be rejected, got success: %s", string(b))
	}
	if !strings.Contains(responses[0].Error.Message, "not found") {
		t.Errorf("expected 'invoice not found' error, got: %s", responses[0].Error.Message)
	}
}

// TestT11_MCPStartupTokenGate verifies P0-4.a: the MCP server must not start
// without a configured mcp_token and a matching presented token. This is the
// decision function used by the `mcp` command's RunE before runMCPServer.
func TestT11_MCPStartupTokenGate(t *testing.T) {
	cases := []struct {
		name      string
		expected  string
		flag      string
		env       string
		wantError bool
	}{
		{"no_setting", "", "tok", "", true},
		{"setting_but_no_presentation", "secret", "", "", true},
		{"setting_flag_wrong", "secret", "WRONG", "", true},
		{"setting_env_wrong", "secret", "", "WRONG", true},
		{"setting_flag_match", "secret", "secret", "", false},
		{"setting_env_match", "secret", "", "secret", false},
		{"flag_preferred_over_env", "secret", "secret", "other", false},
		{"flag_wrong_no_env_fallback", "secret", "WRONG", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := mcpStartupTokenError(c.expected, c.flag, c.env)
			if c.wantError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !c.wantError && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}
