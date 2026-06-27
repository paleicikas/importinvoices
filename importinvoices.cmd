@echo off
cd /d "%~dp0server"
if exist "importinvoices.exe" (
  importinvoices.exe %*
) else (
  go run ./cmd/importinvoices %*
)
