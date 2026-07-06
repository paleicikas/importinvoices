# Importinvoices

https://importinvoices.com/

[![CI](https://github.com/paleicikas/importinvoices/actions/workflows/ci.yml/badge.svg)](https://github.com/paleicikas/importinvoices/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](#license)

Installable, local-first invoice management system powered by AI.

Upload PDF or image invoices, extract structured data with OpenAI or Google Gemini, review them in a web UI, and export to accounting systems (Rivile, i.SAF, Centas, and many more).

## Demo

[![Importinvoices demo](https://img.youtube.com/vi/WzVC35jTgRU/maxresdefault.jpg)](https://youtu.be/WzVC35jTgRU)

## Features

| Area | Details |
|------|---------|
| **Local-first** | Data stays on your machine in SQLite (`~/.importinvoices`) |
| **AI extraction** | OpenAI or Google Gemini with configurable models |
| **Workflow** | Tabbed invoice list: Processing → Ready → Exported |
| **Export** | Built-in templates for JSON, CSV, XML, and Lithuanian accounting systems |
| **VAT Classifiers** | AI-powered VAT code classification with country catalogs (e.g., i-SAF) |
| **Multi-user** | Two roles: admin (manage users + settings) and operator (import/review/export + companies) |
| **Security** | CSRF protection, login rate limiting, session cookies |
| **MCP** | Built-in [Model Context Protocol](https://modelcontextprotocol.io/) server for AI agents (`importinvoices mcp`) |
| **i18n** | Web UI in EN, LT, DE, FR, ES, IT, PL, RU, LV, EE, UK, ZH, JA, KO, AR, HI, PT-BR, ID, VI, TH, HE, FA (22 total) |

## Quick Start

### Installation

**Windows (PowerShell):**

```powershell
iwr -useb https://raw.githubusercontent.com/paleicikas/importinvoices/main/installer/install.ps1 | iex
```

**Linux / macOS (Bash):**

```bash
curl -fsSL https://raw.githubusercontent.com/paleicikas/importinvoices/main/installer/install.sh | bash
```

### First run

Run the interactive setup wizard to create your organization and admin account:

```powershell
importinvoices onboard
```

You can also pass flags: `--org`, `--name`, `--email`, `--password` (minimum 8 characters).

Start the server:

```bash
importinvoices serve
```

Open [http://localhost:8080/](http://localhost:8080/) in your browser.

## CLI commands

| Command | Description |
|---------|-------------|
| `serve` | Start the web server and background invoice worker |
| `onboard` | First-time setup: database, migrations, admin user |
| `reset-password` | Reset a user's password for admin recovery (see "Password recovery" below) |
| `mcp` | Start the MCP server (JSON-RPC over stdin/stdout) |
| `version` | Print the current version |

Global flag: `--data-dir` — override the default data directory (`~/.importinvoices`).

### Password recovery

There is no email-based "forgot password" flow (the app is self-hosted and does not assume an SMTP server). If an admin loses access, reset the password from the server command line:

```bash
importinvoices reset-password --email admin@example.com
```

You will be prompted for a new password (or pass it with `--password`). The command looks up the user by email, updates the bcrypt hash, and **invalidates all existing sessions** for that user (they are logged out everywhere). It works directly against the configured data directory's database, so it must be run on the host that owns the data. `--data-dir` can be used to target a non-default data directory.


## MCP server (AI agents)

The `mcp` command starts a Model Context Protocol server (JSON-RPC over stdin/stdout) that AI agents such as Cursor can connect to. It is **fail-closed**: it will not start without a configured token.

### Setup

1. In the web UI, open **Settings** and set `mcp_token` to a secret value of your choice. This is stored in the app database and is the canonical secret for MCP access.
2. Start the MCP server and present the same token via the `--auth-token` flag **or** the `MCP_AUTH_TOKEN` environment variable:

```bash
importinvoices mcp --auth-token YOUR_TOKEN
# or
MCP_AUTH_TOKEN=YOUR_TOKEN importinvoices mcp
```

If `mcp_token` is not configured, or the presented token does not match, the command exits with an error before serving any request.

### Connecting from Cursor

Add Importinvoices as a command-type MCP server in Cursor (Settings → Features → MCP):

```json
{
  "mcpServers": {
    "importinvoices": {
      "command": "/path/to/importinvoices",
      "args": ["mcp", "--auth-token", "YOUR_TOKEN"]
    }
  }
}
```

### Available tools

- `list_invoices` — list/filter invoices (scoped to the configured organization)
- `get_invoice` — fetch one invoice with line items (cross-org reads are rejected)
- `list_companies` — list vendors/customers
- `list_vat_classifiers` — list VAT codes for the organization
- `import_invoice` — import a file from the staging directory `<data_dir>/mcp-imports/`. The `path` argument must be relative to that directory; absolute paths and `..` traversal are rejected.

## Secure deployment

Importinvoices is designed to run on your own machine or a server you control. For anything beyond a single-user localhost setup, follow this checklist:

1. **Bind to localhost.** The default `http_addr` is `127.0.0.1` on the first free port between 8080 and 8088, so the app is only reachable from the same machine. Do not change this to `0.0.0.0` unless you put a reverse proxy in front.
2. **Put a reverse proxy in front for HTTPS.** Run Caddy, nginx, or Traefik on your domain with TLS, forwarding to `127.0.0.1:8080`. The proxy should forward `Host`, `X-Forwarded-Proto`, and `X-Forwarded-For`. Over HTTPS the app sends `Strict-Transport-Security` automatically.
3. **Add the proxy IP to `trusted_proxies`.** In `config.json`, set `trusted_proxies` to the proxy's IP (e.g. `["127.0.0.1", "::1"]` for a local proxy) so login rate limiting uses the real client IP from `X-Forwarded-For` instead of the proxy's IP.
4. **Complete onboarding immediately.** Run `importinvoices onboard` to create your admin account. The `/setup` page and `/api/v1/setup` endpoint are disabled (403) once the first user exists, so no one else can create an admin on your instance.
5. **Set the MCP token.** If you use the MCP server, set `mcp_token` in **Settings** to a strong secret. The MCP server is fail-closed and will not start without it (see QA.md Q 80a).
6. **Protect the data directory.** `~/.importinvoices` holds the SQLite database (with your API keys), config, and uploaded invoice files. Restrict filesystem permissions to your user, and back it up by copying the whole directory.
7. **Do not expose `/storage/` directly.** Invoice files are served only to authenticated users via `/invoices/{id}/file` (forced as a download) and `/invoices/{id}/preview`; there is no public storage URL.

See QA.md "Security & Data Ownership" for CSRF, rate limiting, session, SSRF, and security-header details.

## Development

### Prerequisites

- [Go 1.26+](https://go.dev/dl/)

### Run from source

```powershell
.\importinvoices.ps1 serve
```

Or from the `server` directory:

```bash
go run ./cmd/importinvoices serve
```

### Build

```bash
cd server
go build -o importinvoices ./cmd/importinvoices
```

### Project layout

```
importinvoices/
├── server/                  # Go backend
│   ├── cmd/importinvoices/  # Entry point
│   └── internal/
│       ├── httpapi/         # REST API & HTTP handlers
│       ├── processor/       # OpenAI & Gemini invoice extraction
│       ├── worker/          # Background invoice processing
│       ├── export/          # Export templates & engine
│       ├── service/         # Business logic
│       └── webui/           # HTML templates & static assets
├── installer/               # install.ps1, install.sh, GoReleaser
├── index.html               # Landing page (21 languages, inline i18n via JS toggle)
├── landing/landing.js       # Landing page language toggle & SEO logic
└── QA.md                    # Detailed Q&A
```

## Testing

The project has **Go unit and integration tests** across core packages. CI runs on every push and pull request to `main`.

```bash
cd server
go test ./...
```

Additional checks (same as CI):

```bash
go vet ./...
golangci-lint run ./...
govulncheck ./...
```

### Test coverage by package

CI enforces a **minimum 15% statement coverage per package** (excluding `domain`, which has no test files). The check runs in `.github/workflows/ci.yml` via `scripts/check_coverage.go`; if any package drops below 15%, CI fails. There is no 100% requirement — coverage is tracked as a floor, not a ceiling, and is expected to grow over time.

| Package | Responsibility | Coverage |
|---------|-----------------|---------:|
| `internal/reqctx` | request context & auth | 100.0% |
| `internal/config` | configuration loading | 90.4% |
| `internal/storage` | file storage & path safety | 88.6% |
| `internal/worker` | background processing | 81.4% |
| `internal/vatcatalog` | VAT country catalogs | 80.0% |
| `internal/db` | SQLite migrations & store | 77.8% |
| `internal/httpapi` | HTTP, CSRF, rate limits | 69.4% |
| `internal/processor` | OpenAI, Gemini, prompts | 69.6% |
| `internal/service` | invoices, companies, auth | 69.2% |
| `internal/webui` | page rendering & FuncMap | 65.4% |
| `internal/export` | templates & formats | 61.9% |
| `internal/cli` | CLI commands & MCP server | 29.9% |
| `internal/media` | file type detection | 20.0% |

**Total coverage: ~47%** across all packages (lower than the per-package numbers because the `cmd/importinvoices` entry point and `internal/testutil` are at 0%). The per-package floor is what CI enforces.

To generate a coverage report:

```bash
cd server
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Documentation

- [QA.md](QA.md) — installation, configuration, export formats, troubleshooting
- [AGENTS.md](AGENTS.md) — notes for AI agents working on this repo

## License

MIT
