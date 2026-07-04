# Agent Instructions

This file contains instructions for AI agents working on this repository.

## Documentation Maintenance

- **QA.md**: Always update `QA.md` whenever there are changes to the system logic, new features are added, or existing behavior is modified.
- **User Clarity**: If you encounter logic or features that might be unclear to a user, proactively add a corresponding Question and Answer to `QA.md`.
- **UI/UX**: When implementing or modifying UI components, ensure they are intuitive. For example, use placeholders in dropdowns and hide context-specific fields until a selection is made to avoid user confusion.
- **Consistency**: Ensure that any changes reflected in `QA.md` are also considered for the FAQ sections in `index.html` and `lt.html` if they are high-level enough for the landing pages.

## MCP Server Integration

- **MCP Support**: The application includes a built-in MCP (Model Context Protocol) server. This allows AI agents to interact with the invoice data.
- **Tools**: When working on the codebase, ensure that any new business logic that should be accessible to AI agents is also exposed via the MCP server in `server/internal/cli/mcp.go`.
- **Auth (fail-closed)**: The MCP server requires a configured `mcp_token` setting (set via the web UI Settings page) and a matching token presented via `--auth-token` or the `MCP_AUTH_TOKEN` env var. Without it, `importinvoices mcp` exits with an error before serving any request. See README "MCP server" section and QA.md Q 80a.
- **Path scope**: `import_invoice` only accepts paths relative to `<data_dir>/mcp-imports/` (the staging dir); absolute paths and `..` traversal are rejected.
- **Org scope**: `get_invoice` / `list_invoices` are scoped to the configured organization; cross-org reads are rejected.
- **Testing**: When testing MCP tools, set `mcp_token` in the test DB via `svc.SetSetting(ctx, "mcp_token", "...")`, then drive `runMCPServer(ctx, svc, expectedToken, stagingDir)` over piped stdin/stdout (see `server/internal/cli/mcp_test.go` for the pipe-swap harness and T-11/T-12/T-13 tests).
