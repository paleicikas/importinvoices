# Agent Instructions

This file contains instructions for AI agents working on this repository.

## Documentation Maintenance

- **QA.md**: Always update `QA.md` whenever there are changes to the system logic, new features are added, or existing behavior is modified.
- **User Clarity**: If you encounter logic or features that might be unclear to a user, proactively add a corresponding Question and Answer to `QA.md`.
- **UI/UX**: When implementing or modifying UI components, ensure they are intuitive. For example, use placeholders in dropdowns and hide context-specific fields until a selection is made to avoid user confusion.
- **Consistency**: Ensure that any changes reflected in `QA.md` are also considered for the FAQ sections in `index.html` (which now hosts all 10 landing languages inline) if they are high-level enough for the landing pages. `lt.html` is now just a redirect stub to `index.html?lang=lt`.

## MCP Server Integration

- **MCP Support**: The application includes a built-in MCP (Model Context Protocol) server. This allows AI agents to interact with the invoice data.
- **Tools**: When working on the codebase, ensure that any new business logic that should be accessible to AI agents is also exposed via the MCP server in `server/internal/cli/mcp.go`.
- **Auth (fail-closed)**: The MCP server requires a configured `mcp_token` setting (set via the web UI Settings page) and a matching token presented via `--auth-token` or the `MCP_AUTH_TOKEN` env var. Without it, `importinvoices mcp` exits with an error before serving any request. See README "MCP server" section and QA.md Q 80a.
- **Path scope**: `import_invoice` only accepts paths relative to `<data_dir>/mcp-imports/` (the staging dir); absolute paths and `..` traversal are rejected.
- **Org scope**: `get_invoice` / `list_invoices` are scoped to the configured organization; cross-org reads are rejected.
- **Testing**: When testing MCP tools, set `mcp_token` in the test DB via `svc.SetSetting(ctx, "mcp_token", "...")`, then drive `runMCPServer(ctx, svc, expectedToken, stagingDir)` over piped stdin/stdout (see `server/internal/cli/mcp_test.go` for the pipe-swap harness and T-11/T-12/T-13 tests).

## Deployment & Security

- **Secure deployment guide**: See README "Secure deployment" section (bind to localhost, reverse proxy with HTTPS, `trusted_proxies`, onboard immediately, set MCP token, protect the data directory). Do not introduce code that serves invoice files from a public path or binds to `0.0.0.0` by default.
- **Encryption at rest is NOT implemented**: The SQLite database (including API keys and the MCP token) and uploaded files are stored in plain form. Do not add marketing or QA claims stating otherwise. Protection is filesystem-level + self-hosted (see QA.md Q 68a).
- **Security & data ownership**: QA.md "Security & Data Ownership" documents CSRF, login rate limiting, session handling, SSRF validation for webhooks/export URLs, security headers, and `Content-Disposition: attachment` for original invoice files. Keep these in sync when changing auth, export, or file-serving code.
