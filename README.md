# Importinvoices

[![CI](https://github.com/paleicikas/importinvoices/actions/workflows/ci.yml/badge.svg)](https://github.com/paleicikas/importinvoices/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](#license)

Installable, local-first invoice management system powered by AI.

Upload PDF or image invoices, extract structured data with OpenAI or Google Gemini, review them in a web UI, and export to accounting systems (Rivile, i.SAF, Centas, and many more).

## Features

| Area | Details |
|------|---------|
| **Local-first** | Data stays on your machine in SQLite (`~/.importinvoices`) |
| **AI extraction** | OpenAI or Google Gemini with configurable models |
| **Workflow** | Tabbed invoice list: Processing → Ready → Exported |
| **Export** | Built-in templates for JSON, CSV, XML, and Lithuanian accounting systems |
| **Security** | CSRF protection, login rate limiting, session cookies |
| **MCP** | Built-in [Model Context Protocol](https://modelcontextprotocol.io/) server for AI agents (`importinvoices mcp`) |
| **i18n** | Web UI in EN, LT, DE, FR, ES, PL, RU, EE |

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
| `mcp` | Start the MCP server (JSON-RPC over stdin/stdout) |
| `version` | Print the current version |

Global flag: `--data-dir` — override the default data directory (`~/.importinvoices`).

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
├── index.html / lt.html     # Landing pages
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

The project enforces **100% statement coverage** for all core packages (excluding `domain` and `cli`). CI will fail if coverage drops below this threshold.

| Package | Tests | Coverage |
|---------|------:|---------:|
| `internal/reqctx` | request context & auth | 100.0% |
| `internal/storage` | file storage & security | 95.0% |
| `internal/config` | configuration loading | 90.4% |
| `internal/db` | SQLite migrations & store | 76.7% |
| `internal/worker` | background processing | 74.5% |
| `internal/httpapi` | HTTP, CSRF, rate limits | 72.6% |
| `internal/processor` | OpenAI, Gemini, prompts | 65.6% |
| `internal/service` | invoices, companies, auth | 65.1% |
| `internal/export` | templates & formats | 61.4% |
| `internal/media` | file type detection | 20.0% |
| `internal/webui` | page rendering | 16.0% |

**Total coverage: 60%+** (and growing). We use `go test -cover` and a custom check script to ensure high quality and reliability.

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
