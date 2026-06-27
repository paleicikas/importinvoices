# Run ImportInvoices CLI from repo root. Usage: .\importinvoices.ps1 onboard
$ErrorActionPreference = "Stop"
Push-Location (Join-Path $PSScriptRoot "server")
try {
    & go run ./cmd/importinvoices @args
} finally {
    Pop-Location
}
