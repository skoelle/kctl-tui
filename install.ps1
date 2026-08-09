# Install script for kctl-tui on Windows.
# Downloads the latest GitHub release binary matching the current architecture
# and installs it to your PATH.
#
# Usage (PowerShell):
#   irm https://raw.githubusercontent.com/skoelle/kctl-tui/main/install.ps1 | iex
#
# Or save and run locally:
#   .\install.ps1
#
# Requires: PowerShell 5.1+, internet access.

$ErrorActionPreference = "Stop"

$Repo = "skoelle/kctl-tui"
$BinName = "kctl-tui"

# --- Detect architecture ---
$arch = $env:PROCESSOR_ARCHITECTURE
switch ($arch) {
    "AMD64"   { $goarch = "amd64" }
    "ARM64"   { $goarch = "arm64" }
    default {
        Write-Error "Unsupported architecture: $arch"
        exit 1
    }
}

# --- Determine install directory ---
$installDir = "$env:USERPROFILE\bin"
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir | Out-Null
}

# Add to PATH if not already there
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$currentPath;$installDir", "User")
    $env:Path = "$env:Path;$installDir"
    Write-Host "Added $installDir to your PATH."
}

# --- Query GitHub API for latest release ---
Write-Host "Detecting latest release for $Repo ..."
try {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
} catch {
    Write-Error "Failed to reach the GitHub API (network error). Check your internet connection and try again."
    exit 1
}

$tag = $release.tag_name
if (-not $tag) {
    Write-Error "Could not find a published release for $Repo. No release has been tagged yet."
    exit 1
}

Write-Host "Latest release: $tag"

# --- Download binary ---
$asset = "kctl-tui-windows-${goarch}.exe"
$url = "https://github.com/$Repo/releases/download/$tag/$asset"
$outFile = "$installDir\$BinName.exe"

Write-Host "Downloading $asset ($tag) ..."
try {
    Invoke-WebRequest -Uri $url -OutFile $outFile -UseBasicParsing
} catch {
    Write-Error "Download failed: $_"
    exit 1
}

Write-Host "Installed $BinName to $outFile"
Write-Host "Done. Run '$BinName' to get started."
