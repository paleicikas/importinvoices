# Importinvoices installer — Windows
# Usage: iwr -useb https://raw.githubusercontent.com/paleicikas/importinvoices/main/installer/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = "paleicikas/importinvoices"
$Binary = "importinvoices"
$InstallDir = if ($env:IMPORTINVOICES_INSTALL_DIR) { $env:IMPORTINVOICES_INSTALL_DIR } else { "$env:LOCALAPPDATA\Programs\importinvoices" }

function Get-LatestVersion {
    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
        return $release.tag_name
    } catch {
        return $null
    }
}

Write-Host "==> Importinvoices installer"

$version = Get-LatestVersion
if (-not $version) {
    Write-Host "No GitHub release found yet. Build from source instead:"
    Write-Host "  cd server; go install ./cmd/importinvoices"
    exit 1
}

$platform = "windows_x86_64"
$url = "https://github.com/$Repo/releases/download/$version/${Binary}_${platform}.zip"
$tmpdir = Join-Path $env:TEMP "importinvoices-install-$(Get-Random)"

New-Item -ItemType Directory -Force -Path $tmpdir | Out-Null
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

Write-Host "==> Downloading $Binary $version ($platform)"
$zip = Join-Path $tmpdir "archive.zip"
Invoke-WebRequest -Uri $url -OutFile $zip
Expand-Archive -Path $zip -DestinationPath $tmpdir -Force

Copy-Item -Force (Join-Path $tmpdir "$Binary.exe") (Join-Path $InstallDir "$Binary.exe")

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
    Write-Host "==> Added $InstallDir to user PATH (restart terminal)"
}

Write-Host "==> Installed to $InstallDir\$Binary.exe"
Write-Host "==> Run: importinvoices serve"
