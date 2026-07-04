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

### 13. Are there password requirements?
Yes. Passwords must be at least **8 characters** long during initial setup, profile password changes, and the `onboard` CLI command.

### 14. How can I skip the confirmation prompt during onboarding?
Use the `--yes` or `-y` flag: `importinvoices onboard -y --email admin@example.com --password secret123`.

### 15. How do I check the current version of the application?
Run `importinvoices version`.

### 16. Can I change the data directory from the command line?
Yes, use the global `--data-dir` flag with any command: `importinvoices --data-dir C:\Invoices serve`.

### 17. What do the `importinvoices.ps1` and `importinvoices.cmd` scripts do?
These are convenience wrappers that run the application from source using `go run`. They are useful for development or if you don't want to build the binary yourself.

### 18. How do I stop the server?
Press `Ctrl+C` in the terminal where the server is running. The application will perform a graceful shutdown.

## Configuration

### 19. Where is the configuration file located?
The configuration is stored in `config.json` inside your data directory (default is `~/.importinvoices/config.json`).

### 20. What settings can I change in `config.json`?
You can configure the data directory, SQLite database file path (`db_path`), HTTP address (port), storage path for files, maximum upload size (`max_upload_bytes`), and trusted reverse proxy IPs (`trusted_proxies`).

### 21. How do I change the port the server runs on?
In `config.json`, edit the `"http_addr"` field (e.g., `"http_addr": "127.0.0.1:9000"`). By default the server binds to `127.0.0.1` on the first free port between 8080 and 8088, so it is only reachable from the same machine.

To expose the app on your local network, set `"http_addr": "0.0.0.0:8080"`. For a public domain with HTTPS, keep the app on `127.0.0.1:8080` and put Caddy or nginx in front as a reverse proxy.

### 22. Can I use a custom domain with HTTPS?
Yes. Run importinvoices on `127.0.0.1:8080` and configure a reverse proxy (Caddy, nginx, Traefik) for your domain. The proxy should forward `Host`, `X-Forwarded-Proto`, and `X-Forwarded-For` headers. Add the proxy's IP to `"trusted_proxies"` in `config.json` (for a local proxy, `["127.0.0.1", "::1"]`) so login rate limiting uses the real client IP from those headers. Export URLs and webhooks will then use your public domain automatically.

### 23. Can I use a database other than SQLite?
No. Importinvoices uses a local **SQLite** database only. By default the file is `data.db` inside your data directory. You can change the path via `"db_path"` in `config.json`.

### 24. What is the default data directory path?
- **Windows:** `C:\Users\<User>\.importinvoices`
- **Unix:** `/home/<user>/.importinvoices`

## AI / LLM

### 25. Which AI providers are supported?
The system currently supports **OpenAI** and **Google Gemini**.

### 26. How do I configure my OpenAI or Google Gemini API key?
Go to **Settings** -> **Artificial Intelligence (LLM)** in the web interface and enter your API key and preferred model.

### 27. Can I use environment variables for API keys?
Yes. If no key is found in the database, the system looks for `OPENAI_API_KEY` or `GOOGLE_API_KEY` environment variables.

### 28. What is the default AI model used?
For OpenAI, the default is `gpt-4o-mini`. For Google Gemini, the default is `gemini-2.5-flash`.

### 29. What data fields does the AI extract from invoices?
The AI extracts document type, invoice number, dates (issue, supply, due), currency, totals (with/without VAT), seller and buyer details (name, code, VAT ID, address, bank accounts), and a detailed list of line items.

### 30. Can I change the AI model for a specific provider?
Yes, you can specify any valid model name (e.g., `gpt-4o`, `gemini-2.5-pro`) in the Settings page.

### 31. Why is my upload blocked with an "LLM not configured" error?
You must provide an API key for at least one AI provider in the Settings page before you can start uploading and processing invoices.

### 32. Does the system use OCR or vision-based models?
The system uses **vision-based models**. Invoices (PDFs or images) are converted to JPEG images and sent directly to the AI model's vision API for data extraction.

## Upload & Processing

### 33. What file formats are supported for invoice uploads?
The system supports **PDF, JPEG, PNG, WEBP, GIF, and TIFF**. Uploads are validated by inspecting the file content (magic bytes), not just the filename or browser-reported content type. If the filename extension does not match the actual file type, the upload is rejected.

### 34. Is there a limit on the number of pages in a PDF?
Yes, the system currently processes the **first 10 pages** of a PDF to ensure optimal performance and cost-efficiency.

### 35. Can I upload multiple files at once?
Yes, the upload interface supports selecting and uploading multiple files simultaneously.

### 36. How does the system handle duplicate invoices?
Duplicate detection happens in two layers. First, the system calculates a SHA-256 hash for every uploaded file; if a file with the same hash already exists in your organization, it is immediately marked as a "Duplicate" and skipped for AI processing. Second, after AI extraction, a **business-key** duplicate check runs: if another non-duplicate invoice in your organization has the same `(seller VAT, invoice series and number)`, the new invoice is also marked as a duplicate of that original — even if the file bytes differ (for example, a re-scan or a differently-encoded PDF of the same invoice). The database enforces this with a unique index on `(org_id, seller_vat, series_and_number)` for non-duplicate invoices. Duplicates display a "Duplicate detected" banner with a link to the original invoice.

### 37. What is the maximum file size for an upload?
The default limit is **10 MB** for the entire upload form (all files combined). You can change this in `~/.importinvoices/config.json` by setting `max_upload_bytes` (value in bytes).

### 38. How does the background processing queue work?
When you upload an invoice, it is added to an internal queue. A background worker processes invoices one by one to avoid overloading the AI API or your server.

### 39. What do the different invoice statuses mean?
- `Pending/Processing`: Waiting for or currently being read by AI.
- `Awaiting confirmation`: AI processing is done; data needs your review.
- `Ready for export`: You have reviewed and confirmed the data.
- `Exported`: The invoice has been exported to an external system.
- `Duplicate`: File already exists in the system.
- `Error`: AI processing failed.

### 40. What is the "Welcome" hero section on the Invoices page?
When you first start using Importinvoices and have no invoices in your system, you will see a special "Welcome" hero section. This section provides a quick way to upload your first invoice and explains the four main steps of the process: Upload, Extract data, Review & Confirm, and Export. Once you upload your first document, this section will be replaced by the standard invoice list table.

### 41. Why are the dashboard status cards colored?
The dashboard status cards use high-visibility background colors to help you quickly identify the state of your invoices: **Amber** for Processing, **Green** for Ready, **Blue** for Export, and **Red** for Errors. This makes them much more noticeable than the standard white cards.

### 42. Why are some badges blue and others gray?
In the navigation tabs, badges that show counts (like "Processing", "Awaiting confirmation", etc.) are displayed in **blue** (`bg-primary`) when the count is greater than zero to draw attention to pending tasks. If the count is zero, the badge is displayed in **gray** (`bg-secondary`). The "Errors" badge is **red** (`bg-danger`) when there are failed invoices.

### 43. How does the system help me focus on invoices that need my attention?
The system uses a visual hierarchy in the navigation tabs. Badges for "Processing", "Awaiting confirmation", and "Ready for export" are highlighted in **blue** when they contain invoices. The "Errors" badge is highlighted in **red** if there are failures. Additionally, a high-visibility alert appears at the top of the Invoices page if you have documents awaiting your review, with a direct "Start review" button to help you process them quickly.

### 44. What are the animated placeholders in the invoice list?
When invoices are in the "Processing" status, the system displays **skeleton loaders** (animated gray placeholders) for data fields that are currently being extracted by AI. This allows you to see the progress and structure of the list while the AI is still reading the documents.

### 45. What is the "Build your partner database" hero section on the Companies page?
Similar to the Invoices page, if you have no companies in your directory yet, you will see a "Build your partner database" hero section. It explains that companies (sellers and buyers) are automatically created when you upload invoices. Our AI extracts partner information to build your directory, allowing you to track purchase and sales history for each company. Once the first company is detected and saved, this section is replaced by the standard company list.

## Review & Confirmation

### 46. How do I review the data extracted by the AI?
Go to the **Invoices** tab and click on an invoice with the status "Awaiting confirmation". You can see the original file side-by-side with the extracted data.

### 47. What should I do if the extracted data is incorrect?
You can manually edit any field in the review interface. Once you are satisfied, click "Confirm" to move it to the "Ready for export" stage.

### 48. How do I confirm an invoice for export?
In the review screen, check the data and click the **Confirm** button. This changes the status to `ready_for_export`.

### 49. Can I reprocess an invoice that failed or has wrong data?
Yes, there is a "Reprocess" option that allows you to send the invoice back to the AI worker for a fresh extraction. If processing fails at any stage (AI error, database save error, missing LLM configuration), the invoice is marked **Error** (`failed`) instead of staying stuck in **Processing**. Your previously saved invoice data and line items remain unchanged until a reprocess completes successfully.

### 49a. What are the allowed invoice status transitions?
Status changes are guarded so the workflow cannot corrupt accounting data. The allowed transitions are:

| Action | Allowed from | Blocked from |
|---|---|---|
| **Confirm** (→ `ready_for_export`) | `processed` | `pending`, `processing`, `failed`, `duplicate`, `exported` |
| **Reprocess** (→ `pending`) | `failed`, `processed`, `ready_for_export` | `duplicate`, `pending`, `processing` |
| **Reprocess exported** (→ `pending`) | `exported` — only with explicit un-export | `exported` without un-export |
| **Edit** (review screen) | `processed`, `failed`, `ready_for_export` | `pending`, `processing`, `duplicate`, `exported` |
| **Export** | `ready_for_export` | any other status |
| **Re-export** | `exported` — only with explicit confirmation | `exported` without confirmation |

Editing an invoice that has already been `exported` is blocked because the data has already been sent to accounting; changing it would diverge the database from your books. Reprocessing an `exported` invoice requires an explicit un-export flag for the same reason. `duplicate` invoices are never re-queued or edited.

### 50. Why are the "Review and confirm" alerts so prominent?
When you have invoices awaiting confirmation, high-visibility blue alerts appear at the top of the Invoices page and the Review page. These are designed to ensure you don't miss any documents that require your attention before they can be exported to your accounting system. Clicking "Start review" or using the actions in the review header allows you to quickly process these documents.

## Export

### 51. What quick export formats are available?
On the **Ready for export** tab, choose **Quick export** and pick a format: **JSON, XML, CSV, or TXT**. This downloads a file immediately without using an accounting template.

### 52. Which accounting systems have prebuilt export templates?
There are 16 prebuilt templates including: **Apskaita5, i.SAF, Agnum, Debetas, Finvalda, Centas, Rivile, Euroskaita, Lobasoft, Paulita, Pragma 3/4, StandardERP, and Saikas**.

### 53. How do I create a custom export template?
Go to **Export Templates** and create a new template. You can define the file structure using Go's `text/template` syntax.

### 54. What templating engine is used for custom exports?
The system uses the standard Go **`text/template`** engine, enriched with custom functions like `xmlEscape`, `formatDate`, and `formatFloat`.

### 55. Can I export invoices directly to an external API?
Yes. You can create an "API" type template where you specify the URL, HTTP method, headers, and a template for the request body.

### 56. How does the system handle multi-file exports?
If an export template generates multiple files (e.g., separate files for customers and invoices), the system automatically packages them into a single **ZIP** archive.

### 57. Can I export both purchase and sales invoices?
Yes. Use **Invoice grouping** in the export toolbar to control how selected invoices are classified in the export payload:
- **Purchases** — all selected invoices are exported into the purchases bucket
- **Sales** — all selected invoices are exported into the sales bucket
- **All** — the system auto-classifies each invoice based on whether the seller or buyer matches your organization

This does **not** filter the invoice list; it only affects how data is grouped inside the exported file.

### 57a. What is the difference between Quick export and Accounting software?
These are two separate export paths on the **Ready for export** tab:
- **Quick export** — built-in JSON, XML, CSV, or TXT download
- **Accounting software** — uses a prebuilt or custom export template (e.g., Debetas CSV, i.SAF XML)

Only one path is active at a time. Template names show the output format in parentheses (e.g., `Debetas (CSV)`, `Rivile (ZIP)`).

### 58. What is the difference between "Suppliers" and "Customers" in the export payload?
In the export data structure, `Suppliers` are companies that issued the invoices (sellers), while `Customers` are the recipients (buyers).

### 58a. What does the "Total" / `total_price` field represent — net or gross?
The line-item `total_price` (shown in the **Total** column on the review page and stored in the `invoice_items.total_price` column) is the **gross amount including VAT** (i.e., `AmountWithVat` = net + VAT). The export payload derives the net amount as `AmountWithoutVat = TotalPrice - VatAmount` and the gross amount as `AmountWithVat = TotalPrice`. This means you do **not** need to manually edit a line after AI processing for the export to contain correct amounts — the worker already stores the gross total. If you do edit a line, enter the gross (with-VAT) total in the **Total** field.

### 58b. Are VAT rates and codes in the export files taken from the invoice, or hardcoded?
They are taken from each invoice line. Export templates render the line's actual VAT rate (`item.VatRate`, e.g. 21 / 9 / 5 / 0) and VAT classifier code (`item.VatClassifier`, e.g. `PVM1`, `PVM2`, `PVM3`, `PVM5`, or a reverse-charge code) dynamically. If a line has no classifier code, the templates fall back to `PVM1` as a default. This applies to all prebuilt accounting templates (i.SAF, Centas, Apskaita5, Finvalda, Pragma4, Euroskaita, Agnum, Saikas, Lobasoft, Paulita). Make sure the VAT classifier on each line is correct before exporting, since the exported percentage and tax code follow that classifier.

### 58c. Which invoices can be exported, and can I re-export an already-exported invoice?
Only invoices with status `ready_for_export` can be exported normally. Invoices that are `pending`, `processing`, `processed` (not yet confirmed), `failed`, or `duplicate` are rejected by the export with a clear error naming the invoice and its status, so unreviewed or failed documents can never reach your accounting software. An invoice that is already `exported` cannot be exported again by default — the export is blocked to prevent duplicate accounting postings. To export an already-exported invoice, go to the **Exported** tab, select the invoices, pick a template, and use the **Re-export selected** button. You will be asked to confirm, because re-exporting may create duplicate postings in your accounting program. The API equivalent is the `allow_re_export: true` field on the `POST /api/v1/export` request body.

### 58d. Are export runs tracked?
Yes. Every export run (including explicit re-exports) is recorded in an `export_batches` audit table with a unique batch ID, the user, the template, the format, the number of invoices, and whether it was a re-export. Each batch is linked to the specific invoice IDs that were exported together via `export_batch_items`. This lets you trace which invoices were exported in the same run and distinguish first exports from re-exports.

### 58e. How are credit notes handled in exports?
Credit notes (invoice type = credit) are exported with **negative amounts**. Both the invoice header totals (net, VAT, gross) and every line's quantity, unit price, net, VAT and gross are negated in the export payload, so accounting templates that sum the lines reduce the totals automatically. The original stored amounts stay positive in the database; only the exported representation is negated.

### 58f. What happens if an invoice's header totals don't match its line items?
Before exporting, the system recomputes each invoice's totals from its line items and compares them with the header totals. If the net, VAT or gross totals disagree by more than EUR 0.01, a reconciliation warning is added to the export result and logged. The export still proceeds (the file is produced and the batch is recorded), so a warning is not a hard failure — it is a signal to review that invoice's amounts. Use it to catch OCR/extraction drift or manual edits that left the header and lines inconsistent.

## Companies & VAT Classifiers

### 59. How are companies managed in the system?
The system automatically extracts company details from invoices. Matching is done by VAT code first, then company code, then name + country. VAT codes are normalized (a leading country prefix such as `LT123456789` is stripped to `123456789`) so the same company is matched once whether or not the invoice printed the prefix, and the database enforces a unique `(organization, VAT code)` constraint. When a seller/buyer has a VAT code or company code and no existing match, a new company is created and the invoice is linked to it. When a seller/buyer has **no VAT code and no company code** and no name match, no junk company is created — the invoice is left unassigned so you can merge it manually.

### 59a. How do I merge two duplicate companies?
Open one of the duplicate companies and use the **"Merge with another company"** panel on the Details tab. Select the company to merge into and confirm. All invoices linked to the first company (as seller or buyer) are re-pointed to the selected company, and the first company is deleted. The selected company's details are kept. This is the recommended way to clean up duplicates that were created before VAT-code normalization was in place, or to consolidate companies that the matcher could not auto-merge.

### 60. What are VAT classifiers and how do I use them?
VAT classifiers (like `PVM1`, `PVM2`) help map invoice VAT rates to your accounting system's requirements. You can manage them in **Settings** -> **VAT classifiers**. You can load default codes for your country (e.g., Lithuania i-SAF codes) or add custom ones. These codes are used by the AI to automatically classify invoice items during processing.

### 61. How do I load default VAT codes for my country?
Go to **Settings** -> **VAT classifiers** and click **Browse catalog**. Select your country to load the standard VAT codes. For Lithuania, we provide the full i-SAF PVM catalog with detailed rules for the AI. For other countries, we provide starter packs with standard rates that you can customize.

### 62. How do I update my VAT codes if the laws change?
Yes. If we update our global catalog, you will see a notification in the VAT classifiers settings. You can click **Add missing** to import only the new codes without affecting your existing custom modifications.

### 63. What are advanced settings and AI rules for VAT classifiers?
Advanced settings allow you to define specific rules for each VAT code to help the AI classify items more accurately. You can provide **Examples** (e.g., "Domestic goods"), set a **Receiving rule** for incoming invoices, an **Issued rule** for outgoing invoices, and specify a **Purchase account**. For Lithuania, you can also toggle **Include in i-SAF** to control which codes are included in the tax report.

### 63a. What happens if the AI assigns a VAT code that is not in my catalog?
The AI is told to pick a code from your organization's active VAT classifiers based on the line's VAT rate and the reverse-charge / receiving / issuing rules; it is no longer hard-coded to map 21% to `PVM1`. After extraction, every line's VAT code is checked against your catalog. If a line uses a code that is not in your catalog, the invoice is flagged with an **"Unknown VAT code"** warning on the review screen showing the offending code(s). You should assign a valid classifier before exporting. The line's VAT rate used in exports is the **classifier's tariff** (the canonical rate from your catalog), not the rate derived from dividing the line sums, so small rounding differences in the derived rate are corrected.

### 63b. Can I delete a VAT classifier that is already used by invoices?
No. Deleting a classifier that is still referenced by any invoice line would orphan those lines and break VAT reporting, so deletion is refused with a message telling you how many lines still use it. Reassign those lines to another classifier first, then delete it.

## Languages & Localization

### 64. Which languages are supported in the user interface?
Importinvoices supports 10 languages: **English, Lithuanian, German, French, Spanish, Italian, Polish, Russian, Latvian, and Estonian**.

### 65. How do I change the UI language?
You can change the language using the selector in the top navigation bar or by setting your preference in the Profile page.

### 66. Does the landing page support automatic language redirection?
No. Users can manually switch between English and Lithuanian versions using the language selector in the navigation bar.

## Security & Data Ownership

### 67. Is my data stored in the cloud?
No. Importinvoices is a **self-hosted** solution. All your invoice files and extracted data stay on your own machine or server.

### 68. How do I back up my data?
Simply copy the entire data directory (default `~/.importinvoices`). It contains the database, the configuration, and all uploaded invoice files.

### 69. Who has access to my API keys?
Your API keys are stored locally in your database. They are only used to communicate with the AI providers (OpenAI or Google) during invoice processing.

### 70. Can I download my uploaded invoice files?
No. Invoice previews and original files are served only to logged-in users of your organization via `/invoices/{id}/preview` and `/invoices/{id}/file`. There is no public `/storage/` URL anymore.

### 71. How is CSRF protection implemented?
All state-changing requests (POST, PUT, PATCH, DELETE) on authenticated routes require a CSRF token. The server sets an HttpOnly cookie named `csrf_token` when you visit setup, login, or any authenticated page. Forms include a hidden `csrf_token` field; JavaScript API calls must send the same value in the `X-CSRF-Token` header. The token is rotated after a successful login. Requests without a matching token receive HTTP 403.

### 72. Is login protected against brute-force attacks?
Yes. The `/api/v1/login` endpoint allows up to **5 failed attempts per client IP** within a **15-minute** window. After that, further login attempts from the same IP receive HTTP **429 Too Many Requests** with a `Retry-After` header. A successful login clears the counter for that IP. The limit is tracked in server memory (per running instance). By default, the client IP comes from the direct connection (`RemoteAddr`). `X-Forwarded-For` and `X-Real-IP` are used only when the request arrives from an IP listed in `"trusted_proxies"`.

### 72a. What URL restrictions apply to webhooks and API export targets?
Webhook URLs must use **HTTPS** and must not resolve to an internal address. Both webhook URLs and API export template URLs are checked against SSRF (server-side request forgery) protections: the hostname is resolved and any IP that is loopback, private (RFC 1918), link-local, multicast, unspecified, or a known cloud metadata endpoint (e.g. `169.254.169.254`) is rejected. This blocks tricks like `169.254.169.254.nip.io` that resolve to the metadata IP. A second check runs at TCP connect time so DNS rebinding between validation and connect is also blocked. Webhook URLs are validated when you save them in your profile, not only when an event fires, so a bad URL is rejected immediately.

### 73. What happens to my sessions when I change my password?
All existing sessions for your account are invalidated immediately. After a profile password change, the server creates a new session for your current browser so you stay logged in. Other browsers or devices must sign in again.

### 73a. Why do I need to enter my current password when changing it?
Changing your password requires your **current password** as confirmation. This prevents someone with momentary access to your logged-in browser from silently changing your password and taking over the account. If you leave the current-password field empty or enter the wrong value, the new password is rejected and your password is not changed.

### 74. Are expired sessions removed from the database?
Yes. Expired sessions are deleted when the server starts, once per hour while it is running, and before new sessions are created.

### 75. What should I do if the server fails to start because the port is in use?
The system tries to find a free port automatically. If you have pinned a port in `config.json` on that is busy, you can either free that port or change the `"http_addr"` setting to a different port.

### 76. Why are my PDF files not being processed correctly?
Ensure the PDF is not password-protected and contains readable text or clear images. If the file is very large, remember that only the first 10 pages are processed.

### 77. I forgot my admin password, how can I reset it?
There is no built-in password reset yet. The `onboard` command cannot be run again after setup. For now, reset the password directly in the database or recreate the data directory if you have no production data yet.

### 78. Why do I need to restart the server after changing `config.json`?
Settings such as HTTP address, database path, and upload limits are read when the server starts. Restart the server after editing `config.json`.

## AI Agents & MCP

### 79. What is MCP and how does Importinvoices use it?
Model Context Protocol (MCP) is an open standard that enables AI models (like those in Cursor, Claude Desktop, or other AI agents) to securely interact with local data and tools. Importinvoices acts as an MCP server, allowing your AI assistant to read, search, and analyze your invoices directly from your local database.

### 80. How do I connect Importinvoices to Cursor?
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

### 80a. How do I configure the MCP token, and why won't the MCP server start without it?
The MCP server is **fail-closed**: it will not start unless an `mcp_token` secret is configured and a matching token is presented. This prevents the server from ever running in an open, unauthenticated state. Setup:
1. Start the web UI and open **Settings** -> set `mcp_token` to a secret value of your choice (store it safely — it is the password for MCP access).
2. Pass the same value to the MCP server via the `--auth-token` flag (as shown in Q 80) **or** via the `MCP_AUTH_TOKEN` environment variable.
3. If the setting is empty, or the presented token does not match, the `mcp` command exits with an error before serving any request.

Per-request, an AI agent may also echo the token in the JSON-RPC `_meta.auth_token` field; if present it must match. A request without `_meta.auth_token` is still accepted because the spawning client already proved knowledge of the token at startup (stdio MCP).

### 81. What can AI Agents do with my invoice data?
When connected via MCP, AI agents can:
- List recent invoices and their statuses.
- Search for specific vendors or amounts.
- Retrieve detailed information about a specific invoice (line items, VAT details, etc.). Access is scoped to the configured organization; invoices belonging to other organizations are not returned.
- Import a new invoice from a file in the MCP imports staging directory (`<data_dir>/mcp-imports/`) and optionally wait for processing to finish. The `path` argument must be relative to that directory; absolute paths and `..` traversal are rejected.
- Help you prepare data for export or answer questions about your spending patterns.

### 82. Is it secure to let AI Agents access my invoices?
The MCP server runs locally on your machine and requires a configured `mcp_token` to start (see Q 80a). You control which AI agents you connect it to. Tool calls are scoped to your organization, and file imports are confined to the MCP imports staging directory. The data never leaves your machine unless you explicitly ask the AI agent to process it (e.g., by asking a question about a specific invoice).

### 82a. Can I access another organization's companies, templates, classifiers, or invoices?
No. Every read, update, and delete is scoped to the organization resolved from your authenticated session. Fetching a company, export template, VAT classifier, or invoice by id only returns it if it belongs to your organization (system export templates are shared and remain readable). A cross-organization request returns "not found" rather than revealing that the record exists in another organization.

### 82b. Can I edit or delete a system export template?
No. System export templates are shared across all organizations and are read-only. Attempting to update or delete one returns "403 Forbidden"; the template is left unchanged. You can still use a system template for exports, and you can clone it into your own organization to create an editable copy.

### 83. Where can I find the source code?
The project is open source and available on GitHub: [https://github.com/paleicikas/importinvoices](https://github.com/paleicikas/importinvoices).

### 83a. What security headers does the application set?
Every response includes `Content-Security-Policy` (restricting scripts/styles/images/fonts to same-origin and inline, blocking object/embed and frame embedding), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, and a `Permissions-Policy` disabling device sensors. Over HTTPS, `Strict-Transport-Security` (HSTS, one year, including subdomains) is also sent so browsers pin to HTTPS.

### 83b. Why is logout a button and not a link?
Logout is a POST form protected by the same CSRF token as the rest of the app, so a malicious page cannot log you out by embedding `<img src="/logout">`. A plain GET `/logout` is no longer accepted. The same CSRF protection applies to flash message cookies, which are now `HttpOnly`, `SameSite=Lax`, and `Secure` over HTTPS.

### 83c. How are downloaded invoice filenames handled safely?
When you download or preview an invoice file, the `Content-Disposition` header is built from the stored filename with all CR/LF/control characters and path separators stripped (preventing header injection), and non-ASCII filenames are encoded per RFC 5987 (`filename*=UTF-8''…`) with an ASCII fallback so Lithuanian characters in filenames download correctly.

### 84. Why are companies not showing up even though I have invoices?
Companies are automatically created when invoices are processed. If you see invoices but no companies, it might be because processing failed to save the company record (for example, a temporary database lock). Reprocess the affected invoice from the review screen to trigger company creation again.

### 85. What should I do if I see "database is locked (5) (SQLITE_BUSY)" errors?
This error occurs when multiple parts of the application try to write to the SQLite database at the same time. Importinvoices limits SQLite to a single connection and waits up to 5 seconds before retrying (`busy_timeout`). Restart the server so it loads the latest version. You can also reprocess affected invoices to recreate missing companies.

### 86. How is the Settings page organized?
The Settings page is organized into four main tabs: **Artificial Intelligence (LLM)**, **Organization**, **AI Agents (MCP)**, and **Export templates**. The tabs are located at the top of the settings area for easy navigation. The first three tabs (LLM, Organization, MCP) are managed on a single page using pills, while "Export templates" is a separate section for managing your export formats. The UI features a consistent tabbed layout across all settings-related pages.

### 87. Why is the "Country" dropdown not selected correctly in the export template edit page?
The "Country" dropdown in the export template edit page uses case-insensitive comparison to ensure that the saved country code (e.g., "Lt") correctly matches the available options in the catalog (e.g., "LT"). This ensures that the correct country is always highlighted when editing a template.
