# Importinvoices Q&A

Comprehensive guide and frequently asked questions for the Importinvoices system.

## Installation

### 1. How do I install Importinvoices on Windows?
Run the following command in PowerShell:
```powershell
iwr -useb https://raw.githubusercontent.com/paleicikas/importinvoices/main/installer/install.ps1 | iex
```
This will download the latest release and install it to `%LOCALAPPDATA%\Programs\importinvoices`.

### 2. How do I install Importinvoices on Linux or macOS?
Run the following command in your terminal:
```bash
curl -fsSL https://raw.githubusercontent.com/paleicikas/importinvoices/main/installer/install.sh | bash
```
The binary will be installed to `~/.local/bin`.

### 3. Can I install the application in a custom directory?
Yes. Set the `IMPORTINVOICES_INSTALL_DIR` environment variable before running the installation script.
- **Windows:** `$env:IMPORTINVOICES_INSTALL_DIR = "C:\MyApps\importinvoices"`
- **Unix:** `export IMPORTINVOICES_INSTALL_DIR="/opt/importinvoices"`

### 4. What are the prerequisites for running Importinvoices?
If you are using the pre-built binaries, there are no external dependencies. The system uses a pure-Go implementation for PDF and image processing. If you are building from source, you need Go 1.26 or later.

### 5. How do I upgrade to the latest version?
Simply run the installation command again. The script will fetch the latest version and overwrite the existing binary.

### 6. How can I install the application from source?
Clone the repository and run the following commands:
```bash
cd server
go build -o importinvoices ./cmd/importinvoices
```
Alternatively, use the provided wrappers in the root directory: `.\importinvoices.ps1` (PowerShell) or `importinvoices.cmd` (Windows CMD).

### 7. How do I uninstall Importinvoices?
- **Windows:** Delete the folder `%LOCALAPPDATA%\Programs\importinvoices` and remove it from your PATH.
- **Unix:** Delete the `importinvoices` binary from `~/.local/bin`.
- **Data:** To remove all data, delete the `~/.importinvoices` directory.

### 8. Is there a way to run the application without installing it globally?
Yes, you can run it directly from the source directory using `go run ./cmd/importinvoices` inside the `server` folder, or by using the `importinvoices.ps1` or `importinvoices.cmd` wrappers in the root folder.

## Running & Commands

### 9. What are the main CLI commands available?
- `serve`: Starts the web server and background worker.
- `onboard`: Runs the interactive setup wizard for the first-time use.
- `version`: Displays the current version of the application.

### 10. How do I start the web server?
Run `importinvoices serve`. By default, it listens on `127.0.0.1` on the first available port between 8080 and 8088.

### 11. What does the `onboard` command do?
It initializes the database, runs migrations, and creates your first organization and administrator account. You can provide details via flags: `--org`, `--name`, `--email`, and `--password`. The password must be at least **8 characters** long. It can only be used while no users exist yet.

### 12. Can I run the web setup again after the first administrator is created?
No. The `/setup` page and `/api/v1/setup` endpoint are available only until the first user exists. After that, visiting `/setup` redirects to login and repeat API calls return `403 Forbidden`.

### 12a. Are there password requirements?
Yes. Passwords must be at least **8 characters** long during initial setup, profile password changes, and the `onboard` CLI command.

### 13. How can I skip the confirmation prompt during onboarding?
Use the `--yes` or `-y` flag: `importinvoices onboard -y --email admin@example.com --password secret123`.

### 13. How do I check the current version of the application?
Run `importinvoices version`.

### 14. Can I change the data directory from the command line?
Yes, use the global `--data-dir` flag with any command: `importinvoices --data-dir C:\Invoices serve`.

### 15. What do the `importinvoices.ps1` and `importinvoices.cmd` scripts do?
These are convenience wrappers that run the application from source using `go run`. They are useful for development or if you don't want to build the binary yourself.

### 16. How do I stop the server?
Press `Ctrl+C` in the terminal where the server is running. The application will perform a graceful shutdown.

## Configuration

### 17. Where is the configuration file located?
The configuration is stored in `config.json` inside your data directory (default is `~/.importinvoices/config.json`).

### 18. What settings can I change in `config.json`?
You can configure the data directory, SQLite database file path (`db_path`), HTTP address (port), storage path for files, maximum upload size (`max_upload_bytes`), and trusted reverse proxy IPs (`trusted_proxies`).

### 19. How do I change the port the server runs on?
In `config.json`, edit the `"http_addr"` field (e.g., `"http_addr": "127.0.0.1:9000"`). By default the server binds to `127.0.0.1` on the first free port between 8080 and 8088, so it is only reachable from the same machine.

To expose the app on your local network, set `"http_addr": "0.0.0.0:8080"`. For a public domain with HTTPS, keep the app on `127.0.0.1:8080` and put Caddy or nginx in front as a reverse proxy.

### 20. Can I use a custom domain with HTTPS?
Yes. Run importinvoices on `127.0.0.1:8080` and configure a reverse proxy (Caddy, nginx, Traefik) for your domain. The proxy should forward `Host`, `X-Forwarded-Proto`, and `X-Forwarded-For` headers. Add the proxy's IP to `"trusted_proxies"` in `config.json` (for a local proxy, `["127.0.0.1", "::1"]`) so login rate limiting uses the real client IP from those headers. Export URLs and webhooks will then use your public domain automatically.

### 21. Can I use a database other than SQLite?
No. Importinvoices uses a local **SQLite** database only. By default the file is `data.db` inside your data directory. You can change the path via `"db_path"` in `config.json`.

### 22. What is the default data directory path?
- **Windows:** `C:\Users\<User>\.importinvoices`
- **Unix:** `/home/<user>/.importinvoices`

## AI / LLM

### 23. Which AI providers are supported?
The system currently supports **OpenAI** and **Google Gemini**.

### 24. How do I configure my OpenAI or Google Gemini API key?
Go to **Settings** -> **Artificial Intelligence (LLM)** in the web interface and enter your API key and preferred model.

### 25. Can I use environment variables for API keys?
Yes. If no key is found in the database, the system looks for `OPENAI_API_KEY` or `GOOGLE_API_KEY` environment variables.

### 26. What is the default AI model used?
For OpenAI, the default is `gpt-4o-mini`. For Google Gemini, the default is `gemini-2.5-flash`.

### 27. What data fields does the AI extract from invoices?
The AI extracts document type, invoice number, dates (issue, supply, due), currency, totals (with/without VAT), seller and buyer details (name, code, VAT ID, address, bank accounts), and a detailed list of line items.

### 28. Can I change the AI model for a specific provider?
Yes, you can specify any valid model name (e.g., `gpt-4o`, `gemini-2.5-pro`) in the Settings page.

### 29. Why is my upload blocked with an "LLM not configured" error?
You must provide an API key for at least one AI provider in the Settings page before you can start uploading and processing invoices.

### 30. Does the system use OCR or vision-based models?
The system uses **vision-based models**. Invoices (PDFs or images) are converted to JPEG images and sent directly to the AI model's vision API for data extraction.

## Upload & Processing

### 31. What file formats are supported for invoice uploads?
The system supports **PDF, JPEG, PNG, WEBP, GIF, and TIFF**. Uploads are validated by inspecting the file content (magic bytes), not just the filename or browser-reported content type. If the filename extension does not match the actual file type, the upload is rejected.

### 32. Is there a limit on the number of pages in a PDF?
Yes, the system currently processes the **first 10 pages** of a PDF to ensure optimal performance and cost-efficiency.

### 33. Can I upload multiple files at once?
Yes, the upload interface supports selecting and uploading multiple files simultaneously.

### 34. How does the system handle duplicate invoices?
The system calculates a SHA-256 hash for every uploaded file. If a file with the same hash already exists in your organization, it is marked as a "Duplicate" and skipped for AI processing to avoid unnecessary API calls.

### 36. What is the maximum file size for an upload?
The default limit is **10 MB** for the entire upload form (all files combined). You can change this in `~/.importinvoices/config.json` by setting `max_upload_bytes` (value in bytes).

### 37. How does the background processing queue work?
When you upload an invoice, it is added to an internal queue. A background worker processes invoices one by one to avoid overloading the AI API or your server.

### 38. What do the different invoice statuses mean?
- `Pending/Processing`: Waiting for or currently being read by AI.
- `Awaiting confirmation`: AI processing is done; data needs your review.
- `Ready for export`: You have reviewed and confirmed the data.
- `Exported`: The invoice has been exported to an external system.
- `Duplicate`: File already exists in the system.
- `Error`: AI processing failed.

### 39. What is the "Welcome" hero section on the Invoices page?
When you first start using Importinvoices and have no invoices in your system, you will see a special "Welcome" hero section. This section provides a quick way to upload your first invoice and explains the four main steps of the process: Upload, Extract data, Review & Confirm, and Export. Once you upload your first document, this section will be replaced by the standard invoice list table.

### 39a. Why are some badges blue and others gray?
In the navigation tabs, badges that show counts (like "Processing", "Awaiting confirmation", etc.) are displayed in **blue** (`bg-primary`) when the count is greater than zero to draw attention to pending tasks. If the count is zero, the badge is displayed in **gray** (`bg-secondary`). The "Errors" badge is **red** (`bg-danger`) when there are failed invoices.

### 39b. How does the system help me focus on invoices that need my attention?
The system uses a visual hierarchy in the navigation tabs. Badges for "Processing", "Awaiting confirmation", and "Ready for export" are highlighted in **blue** when they contain invoices. The "Errors" badge is highlighted in **red** if there are failures. Additionally, a high-visibility alert appears at the top of the Invoices page if you have documents awaiting your review, with a direct "Start review" button to help you process them quickly.

### 39c. What are the animated placeholders in the invoice list?
When invoices are in the "Processing" status, the system displays **skeleton loaders** (animated gray placeholders) for data fields that are currently being extracted by AI. This allows you to see the progress and structure of the list while the AI is still reading the documents.

### 40. What is the "Build your partner database" hero section on the Companies page?
Similar to the Invoices page, if you have no companies in your directory yet, you will see a "Build your partner database" hero section. It explains that companies (sellers and buyers) are automatically created when you upload invoices. Our AI extracts partner information to build your directory, allowing you to track purchase and sales history for each company. Once the first company is detected and saved, this section is replaced by the standard company list.

## Review & Confirmation

### 39. How do I review the data extracted by the AI?
Go to the **Invoices** tab and click on an invoice with the status "Awaiting confirmation". You can see the original file side-by-side with the extracted data.

### 40. What should I do if the extracted data is incorrect?
You can manually edit any field in the review interface. Once you are satisfied, click "Confirm" to move it to the "Ready for export" stage.

### 41. How do I confirm an invoice for export?
In the review screen, check the data and click the **Confirm** button. This changes the status to `ready_for_export`.

### 42. Can I reprocess an invoice that failed or has wrong data?
Yes, there is a "Reprocess" option that allows you to send the invoice back to the AI worker for a fresh extraction. If processing fails at any stage (AI error, database save error, missing LLM configuration), the invoice is marked **Error** (`failed`) instead of staying stuck in **Processing**. Your previously saved invoice data and line items remain unchanged until a reprocess completes successfully.

### 42a. Why are the "Review and confirm" alerts so prominent?
When you have invoices awaiting confirmation, high-visibility blue alerts appear at the top of the Invoices page and the Review page. These are designed to ensure you don't miss any documents that require your attention before they can be exported to your accounting system. Clicking "Start review" or using the actions in the review header allows you to quickly process these documents.

## Export

### 43. What quick export formats are available?
You can quickly export selected invoices to **JSON, XML, CSV, or TXT**.

### 44. Which accounting systems have prebuilt export templates?
There are 16 prebuilt templates including: **Apskaita5, i.SAF, Agnum, Debetas, Finvalda, Centas, Rivile, Euroskaita, Lobasoft, Paulita, Pragma 3/4, StandardERP, and Saikas**.

### 45. How do I create a custom export template?
Go to **Export Templates** and create a new template. You can define the file structure using Go's `text/template` syntax.

### 46. What templating engine is used for custom exports?
The system uses the standard Go **`text/template`** engine, enriched with custom functions like `xmlEscape`, `formatDate`, and `formatFloat`.

### 47. Can I export invoices directly to an external API?
Yes. You can create an "API" type template where you specify the URL, HTTP method, headers, and a template for the request body.

### 48. How does the system handle multi-file exports?
If an export template generates multiple files (e.g., separate files for customers and invoices), the system automatically packages them into a single **ZIP** archive.

### 49. Can I export both purchase and sales invoices?
Yes. The system automatically classifies invoices as "Purchases" or "Sales" based on whether the seller or buyer matches your organization's details. You can filter by type during export.

### 50. What is the difference between "Suppliers" and "Customers" in the export payload?
In the export data structure, `Suppliers` are companies that issued the invoices (sellers), while `Customers` are the recipients (buyers).

## Companies & VAT Classifiers

### 51. How are companies managed in the system?
The system automatically extracts company details from invoices. If a company with the same code or name exists, it is linked; otherwise, a new company record is created.

### 52. What are VAT classifiers and how do I use them?
VAT classifiers (like `PVM1`, `PVM2`) help map invoice VAT rates to your accounting system's requirements. You can define rules in Settings to automatically assign these based on VAT percentage and type.

### 53. Can I link extracted invoices to existing companies in my database?
Yes, the AI tries to match extracted names and codes against your existing company list to maintain data consistency.

### 54. Can I delete a company from the list?
Yes. On the Companies page, companies with **no linked purchase or sales invoices** show a delete button. Deletion requires confirmation and a valid CSRF token. Companies linked to invoices cannot be deleted.

## Languages & Localization

### 55. Which languages are supported in the user interface?
Importinvoices supports 10 languages: **English, Lithuanian, German, French, Spanish, Italian, Polish, Russian, Latvian, and Estonian**.

### 56. How do I change the UI language?
You can change the language using the selector in the top navigation bar or by setting your preference in the Profile page.

### 57. Does the landing page support automatic language redirection?
No. Users can manually switch between English and Lithuanian versions using the language selector in the navigation bar.

## Security & Data Ownership

### 58. Is my data stored in the cloud?
No. Importinvoices is a **self-hosted** solution. All your invoice files and extracted data stay on your own machine or server.

### 59. How do I back up my data?
Simply copy the entire data directory (default `~/.importinvoices`). It contains the database, the configuration, and all uploaded invoice files.

### 60. Who has access to my API keys?
Your API keys are stored locally in your database. They are only used to communicate with the AI providers (OpenAI or Google) during invoice processing.

### 61. Can anyone download my uploaded invoice files?
No. Invoice previews and original files are served only to logged-in users of your organization via `/invoices/{id}/preview` and `/invoices/{id}/file`. There is no public `/storage/` URL anymore.

### 62. How is CSRF protection implemented?
All state-changing requests (POST, PUT, PATCH, DELETE) on authenticated routes require a CSRF token. The server sets an HttpOnly cookie named `csrf_token` when you visit setup, login, or any authenticated page. Forms include a hidden `csrf_token` field; JavaScript API calls must send the same value in the `X-CSRF-Token` header. The token is rotated after a successful login. Requests without a matching token receive HTTP 403.

### 63. Is login protected against brute-force attacks?
Yes. The `/api/v1/login` endpoint allows up to **5 failed attempts per client IP** within a **15-minute** window. After that, further login attempts from the same IP receive HTTP **429 Too Many Requests** with a `Retry-After` header. A successful login clears the counter for that IP. The limit is tracked in server memory (per running instance). By default, the client IP comes from the direct connection (`RemoteAddr`). `X-Forwarded-For` and `X-Real-IP` are used only when the request arrives from an IP listed in `"trusted_proxies"`.

### 63a. What happens to my sessions when I change my password?
All existing sessions for your account are invalidated immediately. After a profile password change, the server creates a new session for your current browser so you stay logged in. Other browsers or devices must sign in again.

### 63b. Are expired sessions removed from the database?
Yes. Expired sessions are deleted when the server starts, once per hour while it is running, and before new sessions are created.

### 64. What should I do if the server fails to start because the port is in use?
The system tries to find a free port automatically. If you have pinned a port in `config.json` that is busy, you can either free that port or change the `"http_addr"` setting to a different port.

### 65. Why are my PDF files not being processed correctly?
Ensure the PDF is not password-protected and contains readable text or clear images. If the file is very large, remember that only the first 10 pages are processed.

### 66. I forgot my admin password, how can I reset it?
There is no built-in password reset yet. The `onboard` command cannot be run again after setup. For now, reset the password directly in the database or recreate the data directory if you have no production data yet.

### 67. Why do I need to restart the server after changing `config.json`?
Settings such as HTTP address, database path, and upload limits are read when the server starts. Restart the server after editing `config.json`.

## AI Agents & MCP

### 68. What is MCP and how does Importinvoices use it?
Model Context Protocol (MCP) is an open standard that enables AI models (like those in Cursor, Claude Desktop, or other AI agents) to securely interact with local data and tools. Importinvoices acts as an MCP server, allowing your AI assistant to read, search, and analyze your invoices directly from your local database.

### 69. How do I connect Importinvoices to Cursor?
You can add Importinvoices as an MCP server in Cursor settings:
1. Open **Cursor Settings** -> **Features** -> **MCP**.
2. Click **+ Add New MCP Server**.
3. Name: `Importinvoices`
4. Type: `command`
5. Command (Windows): `C:\path\to\importinvoices\importinvoices.cmd mcp --auth-token YOUR_TOKEN_HERE`

Alternatively, you can add it manually to your `%USERPROFILE%\.cursor\mcp.json` file:
```json
{
  "mcpServers": {
    "importinvoices": {
      "command": "C:\\path\\to\\importinvoices\\importinvoices.cmd",
      "args": ["mcp", "--auth-token", "YOUR_TOKEN_HERE"]
    }
  }
}
```
*Note: On Windows, use the full path to `importinvoices.cmd`. If you installed the pre-built binary, you can also use `%LOCALAPPDATA%\\Programs\\importinvoices\\importinvoices.exe` with args `["mcp", "--auth-token", "YOUR_TOKEN_HERE"]`.*

### 70. What can AI Agents do with my invoice data?
When connected via MCP, AI agents can:
- List recent invoices and their statuses.
- Search for specific vendors or amounts.
- Retrieve detailed information about a specific invoice (line items, VAT details, etc.).
- Import a new invoice from a local file path and optionally wait for processing to finish.
- Help you prepare data for export or answer questions about your spending patterns.

### 71. Is it secure to let AI Agents access my invoices?
Yes. The MCP server runs locally on your machine. You control which AI agents you connect it to. The data never leaves your machine unless you explicitly ask the AI agent to process it (e.g., by asking a question about a specific invoice).

### 72. Where can I find the source code?
The project is open source and available on GitHub: [https://github.com/paleicikas/importinvoices](https://github.com/paleicikas/importinvoices).

### 73. How is the Settings page organized?
The Settings page is organized into four main tabs: **Artificial Intelligence (LLM)**, **Organization**, **AI Agents (MCP)**, and **Export templates**. The tabs are located at the top of the settings area for easy navigation. The first three tabs (LLM, Organization, MCP) are managed on a single page using pills, while "Export templates" is a separate section for managing your export formats. The UI features a consistent tabbed layout across all settings-related pages.
