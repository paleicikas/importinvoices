# Importinvoices installer — Windows
# Usage: iwr -useb https://raw.githubusercontent.com/paleicikas/importinvoices/main/installer/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = "paleicikas/importinvoices"
$Binary = "importinvoices"
$InstallDir = if ($env:IMPORTINVOICES_INSTALL_DIR) { $env:IMPORTINVOICES_INSTALL_DIR } else { "$env:LOCALAPPDATA\Programs\importinvoices" }

function Get-LatestRelease {
    try {
        $releases = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases"
        foreach ($release in $releases) {
            if ($release.assets.Count -gt 0) {
                return $release
            }
        }
        return $null
    } catch {
        return $null
    }
}

Write-Host "==> Importinvoices installer"

$release = Get-LatestRelease
if (-not $release) {
    Write-Host "No GitHub release with binaries found yet. Build from source instead:"
    Write-Host "  cd server; go install ./cmd/importinvoices"
    exit 1
}

$version = $release.tag_name
$platform = "windows_x86_64"
$assetName = "${Binary}_${platform}.zip"
$asset = $release.assets | Where-Object { $_.name -eq $assetName }

if (-not $asset) {
    Write-Host "No binary found for $platform in release $version. Build from source instead:"
    Write-Host "  cd server; go install ./cmd/importinvoices"
    exit 1
}

$url = $asset.browser_download_url
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
