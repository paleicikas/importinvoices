package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

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

		// Validate token if provided in flag
		expectedToken, _ := svc.GetSetting(cmd.Context(), "mcp_token")
		if expectedToken != "" && mcpToken != "" && mcpToken != expectedToken {
			return fmt.Errorf("invalid MCP token")
		}

		return runMCPServer(cmd.Context(), svc)
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

func runMCPServer(ctx context.Context, svc *service.Service) error {
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
			res.Result = map[string]any{
				"tools": []map[string]any{
					{
						"name":        "list_invoices",
						"description": "List and filter invoices",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"limit":  map[string]any{"type": "integer", "default": 10},
								"search": map[string]any{"type": "string", "description": "Text search in filename, seller, buyer or number"},
								"tab":    map[string]any{"type": "string", "description": "Filter by status tab", "enum": []string{"all", "processing", "ready", "export", "exported", "failed", "duplicates"}},
								"filters": map[string]any{
									"type":        "object",
									"description": "Column filters. Keys can be column IDs (0-16) or field names (e.g., 'seller_name', 'status', 'currency'). Values are arrays of strings.",
									"properties": map[string]any{
										"0":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "created_at"},
										"1":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "series_and_number"},
										"2":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "type"},
										"3":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "issue_date"},
										"4":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "supply_date"},
										"5":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "payment_due_date"},
										"6":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "seller_name"},
										"7":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "seller_code"},
										"8":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "seller_vat"},
										"9":                 map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "buyer_name"},
										"10":                map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "buyer_code"},
										"11":                map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "buyer_vat"},
										"12":                map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "amount_without_vat"},
										"13":                map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "vat_amount"},
										"14":                map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "amount_with_vat"},
										"15":                map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "currency"},
										"16":                map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "status"},
										"seller_name":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
										"series_and_number": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
										"status":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
										"currency":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
									},
								},
							},
						},
					},
					{
						"name":        "get_invoice",
						"description": "Get detailed information about a specific invoice",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id": map[string]any{"type": "string"},
							},
							"required": []string{"id"},
						},
					},
					{
						"name":        "list_companies",
						"description": "List companies (vendors and customers)",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"search": map[string]any{"type": "string"},
							},
						},
					},
					{
						"name":        "import_invoice",
						"description": "Import an invoice from a local file path",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"path": map[string]any{"type": "string", "description": "Absolute path to the invoice file (PDF, JPG, PNG)"},
								"wait": map[string]any{"type": "boolean", "description": "Wait for processing to complete", "default": false},
							},
							"required": []string{"path"},
						},
					},
				},
			}
		case "tools/call":
			var params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				res.Error = &JSONRPCError{Code: -32602, Message: "Invalid params"}
			} else {
				result, err := callTool(ctx, svc, params.Name, params.Arguments)
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

func callTool(ctx context.Context, svc *service.Service, name string, args json.RawMessage) (any, error) {
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

		columnMap := map[string]int{
			"created_at":         0,
			"series_and_number":  1,
			"type":               2,
			"issue_date":         3,
			"supply_date":        4,
			"payment_due_date":   5,
			"seller_name":        6,
			"seller_code":        7,
			"seller_vat":         8,
			"buyer_name":         9,
			"buyer_code":         10,
			"buyer_vat":          11,
			"amount_without_vat": 12,
			"vat_amount":         13,
			"amount_with_vat":    14,
			"currency":           15,
			"status":             16,
		}

		colFilters := make(map[int][]string)
		for k, v := range params.Filters {
			// Try to parse as int ID first
			var colID int
			if _, err := fmt.Sscanf(k, "%d", &colID); err == nil {
				colFilters[colID] = v
			} else if id, ok := columnMap[k]; ok {
				// Otherwise use name mapping
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

		f, err := os.Open(params.Path)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		// Get first user and org
		var userID string
		err = svc.Store().DB().QueryRowContext(ctx, "SELECT id FROM users LIMIT 1").Scan(&userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}
		org, err := svc.GetOrganization(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get organization: %w", err)
		}

		inv, err := svc.ImportInvoice(ctx, userID, org.ID, filepath.Base(params.Path), f)
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
	default:
		return nil, fmt.Errorf("tool not found: %s", name)
	}
}

func mustMarshal(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
