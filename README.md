# Importinvoices

Installable, local-first invoice management system powered by AI.

## Features

- **Local-first:** Your data stays on your machine in a SQLite database.
- **AI-powered:** Automatic invoice data extraction using OpenAI or Google Gemini.
- **Tabbed List:** Easily manage invoices through different stages (Processing, Ready, Exported).
- **Export:** Quick export to JSON, CSV, and XML.

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

### Running

```bash
importinvoices serve
```

### Onboarding (CLI)

Run the interactive wizard to set up your organization and admin account:
```powershell
.\importinvoices.ps1 onboard
```
(You can also provide details via flags: `--org`, `--name`, `--email`, `--password`)

Then open [http://localhost:8080/](http://localhost:8080/) in your browser.

## Development

### Prerequisites

- Go 1.26+

### Run from source

```powershell
.\importinvoices.ps1 serve
```

### Build

```bash
cd server
go build -o importinvoices ./cmd/importinvoices
```

## License

MIT
