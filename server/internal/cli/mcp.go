package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/paleicikas/importinvoices/server/internal/config"
	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/media"
	"github.com/paleicikas/importinvoices/server/internal/service"
	"github.com/paleicikas/importinvoices/server/internal/storage"
	"github.com/paleicikas/importinvoices/server/internal/worker"
	"github.com/spf13/cobra"
	"time"
)

var (
	mcpToken  string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the importinvoices MCP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Resolve(dataDir)
		if err != nil {
			return err
		}

		store, err := db.Open(cfg.DBPath)
		if err != nil {
			return err
		}
		defer store.Close()

		strg, err := storage.New(cfg.StoragePath)
		if err != nil {
			return err
		}

		mediaSvc := media.New(filepath.Join(cfg.DataDir, "temp"))
		svc := service.New(store, strg, mediaSvc)
		w := worker.New(store, svc, mediaSvc)
		svc.SetWorker(w)

		go w.Start(cmd.Context())

		// Fail closed: MCP must not start without a configured token.
		// The mcp_token setting is the canonical secret stored in the DB; the
		// --auth-token flag is how the caller presents it. Both must be present
		// and must match.
		expectedToken, _ := svc.GetSetting(cmd.Context(), "mcp_token")
		if err := mcpStartupTokenError(expectedToken, mcpToken, os.Getenv("MCP_AUTH_TOKEN")); err != nil {
			return err
		}

		// MCP imports are restricted to a staging directory under the data dir.
		// The MCP client must place files there first, then call import_invoice
		// with a relative path; absolute paths and traversal are rejected.
		stagingDir := filepath.Join(cfg.DataDir, "mcp-imports")
		if err := os.MkdirAll(stagingDir, 0o755); err != nil {
			return fmt.Errorf("failed to create MCP imports directory: %w", err)
		}

		return runMCPServer(cmd.Context(), svc, expectedToken, stagingDir)
	},
}

func init() {
	mcpCmd.Flags().StringVar(&mcpToken, "auth-token", "", "Authentication token for MCP")
	rootCmd.AddCommand(mcpCmd)
}

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// toolDef describes one MCP tool advertised via tools/list. Centralising the
// registry here removes the previous inline literal in the tools/list handler
// (a single hand-maintained []map[string]any) and lets the filter schema be
// built from the service.InvoiceColumnIndexByName registry instead of a
// hand-synced mix of numeric column ids and field-name keys.
type toolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
}

func mcpTools() []toolDef {
	return []toolDef{
		{
			Name:        "list_invoices",
			Description: "List and filter invoices",
			InputSchema: listInvoicesSchema(),
		},
		{
			Name:        "get_invoice",
			Description: "Get detailed information about a specific invoice",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"id": map[string]any{"type": "string"}},
				"required":   []string{"id"},
			},
		},
		{
			Name:        "list_companies",
			Description: "List companies (vendors and customers)",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"search": map[string]any{"type": "string"}},
			},
		},
		{
			Name:        "import_invoice",
			Description: "Import an invoice from a local file path",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Path to the invoice file (PDF, JPG, PNG), relative to the MCP imports staging directory"},
					"wait": map[string]any{"type": "boolean", "description": "Wait for processing to complete", "default": false},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "list_vat_classifiers",
			Description: "List VAT classifiers (PVM codes) for the organization",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

func listInvoicesSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit":  map[string]any{"type": "integer", "default": 10},
			"search": map[string]any{"type": "string", "description": "Text search in filename, seller, buyer or number"},
			"tab":    map[string]any{"type": "string", "description": "Filter by status tab", "enum": []string{"all", "processing", "ready", "export", "exported", "failed", "duplicates"}},
			"filters": invoiceFilterSchema(),
		},
	}
}

// invoiceFilterSchema builds the list_invoices `filters` property from the
// service.InvoiceColumnIndexByName registry. Only named field-name keys are
// advertised — numeric column ids were dropped from the public schema so the
// MCP contract matches the UI/SQL column names exactly.
func invoiceFilterSchema() map[string]any {
	props := make(map[string]any, len(service.InvoiceColumnIndexByName))
	for name := range service.InvoiceColumnIndexByName {
		props[name] = map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	}
	return map[string]any{
		"type":        "object",
		"description": "Column filters keyed by field name (e.g. 'seller_name', 'status', 'currency', 'vat_codes'). Values are arrays of strings.",
		"properties":  props,
	}
}

func runMCPServer(ctx context.Context, svc *service.Service, expectedToken, stagingDir string) error {
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)

	for {
		var req JSONRPCRequest
		if err := dec.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		// JSON-RPC notifications omit the id field and must not receive a response.
		if len(req.ID) == 0 {
			continue
		}

		var res JSONRPCResponse
		res.JSONRPC = "2.0"
		res.ID = req.ID

		switch req.Method {
		case "initialize":
			res.Result = map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]string{
					"name":    "importinvoices",
					"version": "1.0.0",
				},
			}
		case "ping":
			res.Result = map[string]any{}
	case "tools/list":
		tools := make([]map[string]any, 0, len(mcpTools()))
		for _, td := range mcpTools() {
			tools = append(tools, map[string]any{
				"name":        td.Name,
				"description": td.Description,
				"inputSchema": td.InputSchema,
			})
		}
		res.Result = map[string]any{"tools": tools}
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
			Meta      struct {
				AuthToken string `json:"auth_token"`
			} `json:"_meta"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			res.Error = &JSONRPCError{Code: -32602, Message: "Invalid params"}
		} else if params.Meta.AuthToken != "" && params.Meta.AuthToken != expectedToken {
			// Per-request defense-in-depth: when a client presents a token in
			// _meta.auth_token, it must match the configured mcp_token. A
			// missing per-request token is allowed because the session was
			// already authenticated at startup (stdio MCP: the spawning client
			// proved knowledge of the token via --auth-token / MCP_AUTH_TOKEN).
			res.Error = &JSONRPCError{Code: -32001, Message: "Unauthorized: invalid MCP token"}
		} else {
			result, err := callTool(ctx, svc, params.Name, params.Arguments, stagingDir)
			if err != nil {
				res.Error = &JSONRPCError{Code: -32603, Message: err.Error()}
			} else {
				res.Result = result
			}
		}
		default:
			res.Error = &JSONRPCError{Code: -32601, Message: "Method not found"}
		}

		if err := enc.Encode(res); err != nil {
			return err
		}
	}
}

func callTool(ctx context.Context, svc *service.Service, name string, args json.RawMessage, stagingDir string) (any, error) {
	switch name {
	case "list_invoices":
		var params struct {
			Limit   int                 `json:"limit"`
			Search  string              `json:"search"`
			Tab     string              `json:"tab"`
			Filters map[string][]string `json:"filters"`
		}
		_ = json.Unmarshal(args, &params)
		if params.Limit <= 0 {
			params.Limit = 10
		}

		colFilters := make(map[int][]string)
		// Public schema advertises only named field keys (see invoiceFilterSchema);
		// numeric column ids are no longer accepted.
		for k, v := range params.Filters {
			if id, ok := service.InvoiceColumnIndexByName[k]; ok {
				colFilters[id] = v
			}
		}

		invoices, _, err := svc.ListInvoices(ctx, service.InvoiceListParams{
			Limit:         params.Limit,
			Search:        params.Search,
			Tab:           params.Tab,
			ColumnFilters: colFilters,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"content": []map[string]any{{
			"type": "text",
			"text": mustMarshal(invoices),
		}}}, nil
	case "get_invoice":
		var params struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, err
		}
		inv, items, err := svc.GetInvoice(ctx, params.ID)
		if err != nil {
			return nil, err
		}
		// Org scope: reject cross-org reads without revealing the invoice exists.
		org, err := svc.GetOrganization(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get organization: %w", err)
		}
		if inv.OrgID != org.ID {
			return nil, fmt.Errorf("invoice not found")
		}
		return map[string]any{"content": []map[string]any{{
			"type": "text",
			"text": mustMarshal(map[string]any{"invoice": inv, "items": items}),
		}}}, nil
	case "list_companies":
		var params struct {
			Search string `json:"search"`
		}
		_ = json.Unmarshal(args, &params)
		org, err := svc.GetOrganization(ctx)
		if err != nil {
			return nil, err
		}
		companies, err := svc.ListCompanies(ctx, org.ID, service.CompanyListParams{Search: params.Search})
		if err != nil {
			return nil, err
		}
		return map[string]any{"content": []map[string]any{{
			"type": "text",
			"text": mustMarshal(companies),
		}}}, nil
	case "import_invoice":
		var params struct {
			Path string `json:"path"`
			Wait bool   `json:"wait"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, err
		}

		absPath, err := resolveMCPImportPath(stagingDir, params.Path)
		if err != nil {
			return nil, err
		}
		f, err := os.Open(absPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		// Get first user and org
		user, err := svc.DefaultUser(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get default user: %w", err)
		}
		org, err := svc.GetOrganization(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get organization: %w", err)
		}

		inv, err := svc.ImportInvoice(ctx, user.ID, org.ID, filepath.Base(params.Path), f)
		if err != nil {
			return nil, err
		}

		if params.Wait {
			// Wait for processing to complete
			for i := 0; i < 60; i++ {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(1 * time.Second):
					updatedInv, items, err := svc.GetInvoice(ctx, inv.ID)
					if err != nil {
						return nil, err
					}
					if updatedInv.Status == "processed" || updatedInv.Status == "ready_for_export" {
						return map[string]any{"content": []map[string]any{{
							"type": "text",
							"text": mustMarshal(map[string]any{"invoice": updatedInv, "items": items}),
						}}}, nil
					}
					if updatedInv.Status == "failed" {
						return nil, fmt.Errorf("processing failed")
					}
					if updatedInv.Status == "duplicate" {
						return map[string]any{"content": []map[string]any{{
							"type": "text",
							"text": mustMarshal(map[string]any{"invoice": updatedInv, "items": items, "message": "Duplicate invoice"}),
						}}}, nil
					}
				}
			}
			return nil, fmt.Errorf("processing timed out")
		}

		return map[string]any{"content": []map[string]any{{
			"type": "text",
			"text": mustMarshal(inv),
		}}}, nil
	case "list_vat_classifiers":
		org, err := svc.GetOrganization(ctx)
		if err != nil {
			return nil, err
		}
		classifiers, err := svc.ListVatClassifiers(ctx, org.ID, 0)
		if err != nil {
			return nil, err
		}
		return map[string]any{"content": []map[string]any{{
			"type": "text",
			"text": mustMarshal(classifiers),
		}}}, nil
	default:
		return nil, fmt.Errorf("tool not found: %s", name)
	}
}

func mustMarshal(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

// mcpStartupTokenError enforces the fail-closed startup gate. The MCP server
// must not start unless the mcp_token setting is configured AND a matching
// token is presented via the --auth-token flag or MCP_AUTH_TOKEN env var.
// Returns nil only when expectedToken and presentedToken are both non-empty
// and equal.
func mcpStartupTokenError(expectedToken, flagToken, envToken string) error {
	if expectedToken == "" {
		return fmt.Errorf("MCP token not configured: set mcp_token in Settings before starting the MCP server")
	}
	presentedToken := flagToken
	if presentedToken == "" {
		presentedToken = envToken
	}
	if presentedToken == "" {
		return fmt.Errorf("MCP token not provided: pass --auth-token (or set MCP_AUTH_TOKEN env var) matching the configured mcp_token")
	}
	if presentedToken != expectedToken {
		return fmt.Errorf("invalid MCP token")
	}
	return nil
}

// resolveMCPImportPath confines an MCP import_invoice path to the staging
// directory. It rejects empty paths, absolute paths (Unix or Windows), drive
// letters, leading separators, and any traversal that would escape the
// staging root. The returned path is the cleaned absolute path inside
// stagingDir.
func resolveMCPImportPath(stagingDir, requested string) (string, error) {
	if requested == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(requested) {
		return "", fmt.Errorf("absolute paths are not allowed; place the file in the MCP imports directory and pass a relative path")
	}
	// Reject leading separators (e.g. "/etc/passwd" on Windows is not IsAbs but
	// is still a root-relative path) and Windows drive letters (e.g. "C:\\...").
	if strings.HasPrefix(requested, "/") || strings.HasPrefix(requested, string(filepath.Separator)) {
		return "", fmt.Errorf("absolute paths are not allowed; place the file in the MCP imports directory and pass a relative path")
	}
	if len(requested) >= 2 && requested[1] == ':' && isASCIILetter(requested[0]) {
		return "", fmt.Errorf("absolute paths are not allowed; place the file in the MCP imports directory and pass a relative path")
	}
	clean := filepath.Clean(requested)
	for _, seg := range strings.Split(filepath.ToSlash(clean), "/") {
		if seg == ".." {
			return "", fmt.Errorf("path traversal (..) is not allowed")
		}
	}
	abs := filepath.Join(stagingDir, clean)
	rel, err := filepath.Rel(stagingDir, abs)
	if err != nil || rel == ".." || strings.HasPrefix(filepath.ToSlash(rel), "../") {
		return "", fmt.Errorf("path escapes the MCP imports directory")
	}
	return abs, nil
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
