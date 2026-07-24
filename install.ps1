# Dispatch installer for Windows (PowerShell 5.1+).
#
# Usage:
#   irm https://raw.githubusercontent.com/iftimiemarius/dispatch/main/install.ps1 | iex
#
# Downloads the latest Dispatch release for your architecture from GitHub
# Releases, verifies the SHA-256 checksum, and installs dispatch.exe to
# %LOCALAPPDATA%\dispatch (added to the user PATH).
#
# To install a specific version:
#   $env:DISPATCH_VERSION = "v0.1.0"; irm .../install.ps1 | iex
# To choose the install dir:
#   $env:DISPATCH_INSTALL_DIR = "C:\Tools\dispatch"; irm .../install.ps1 | iex

# Exit on any error.
$ErrorActionPreference = "Stop"

$Repo = "iftimiemarius/dispatch"
$InstallDir = if ($env:DISPATCH_INSTALL_DIR) { $env:DISPATCH_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "dispatch" }
$Version = $env:DISPATCH_VERSION  # empty = latest

function Info($msg)  { Write-Host "   - $msg" }
function Fatal($msg) { Write-Host "   x $msg" -ForegroundColor Red; exit 1 }

# --- detect architecture ----------------------------------------------------

# PROCESSOR_ARCHITECTURE is AMD64 on x64, ARM64 on aarch64.
switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { $GoArch = "amd64" }
    "ARM64" { $GoArch = "arm64" }
    default { Fatal "unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
}

Write-Host ""
Write-Host "  Dispatch installer"
Write-Host "  -------------------"
Info "platform: windows/$GoArch"
Info "install dir: $InstallDir"
if ($Version) { Info "version: $Version" } else { Info "version: latest" }

# --- resolve the release to install -----------------------------------------

$ApiBase = "https://api.github.com/repos/$Repo/releases"
$ReleaseUrl = if ($Version) { "$ApiBase/tags/$Version" } else { "$ApiBase/latest" }

Info "fetching release metadata..."
try {
    $Headers = @{ "Accept" = "application/vnd.github+json" }
    # PowerShell on Windows uses TLS; ensure modern TLS is enabled.
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $Release = Invoke-RestMethod -Uri $ReleaseUrl -Headers $Headers -UseBasicParsing
} catch {
    Fatal "failed to fetch release metadata: $_"
}

$ReleaseTag = $Release.tag_name
if (-not $ReleaseTag) { Fatal "could not determine release tag" }
Info "release: $ReleaseTag"

# --- find the matching asset + checksums file -------------------------------

# Tag may or may not have a leading 'v'; asset names strip it in our GoReleaser config.
$VerNoV = $ReleaseTag.TrimStart("v")
$AssetName = "dispatch_$VerNoV`_windows_$GoArch.zip"
$Asset = $Release.assets | Where-Object { $_.name -eq $AssetName } | Select-Object -First 1
if (-not $Asset) { Fatal "no release asset matched $AssetName for windows/$GoArch" }
Info "asset: $AssetName"

$ChecksumAsset = $Release.assets | Where-Object { $_.name -eq "checksums.txt" } | Select-Object -First 1
if (-not $ChecksumAsset) { Fatal "checksums.txt asset not found in release" }

# --- download, verify, extract ----------------------------------------------

$Tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $Tmp -Force | Out-Null

try {
    $ArchivePath = Join-Path $Tmp $AssetName
    $ChecksumPath = Join-Path $Tmp "checksums.txt"

    Info "downloading archive..."
    Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $ArchivePath -UseBasicParsing

    Info "downloading checksums..."
    Invoke-WebRequest -Uri $ChecksumAsset.browser_download_url -OutFile $ChecksumPath -UseBasicParsing

    # Verify SHA-256.
    $Expected = $null
    foreach ($line in (Get-Content $ChecksumPath)) {
        if ($line -match "(?i)^([0-9a-f]{64})\s+\*?(.+)$") {
            if ($Matches[2] -eq $AssetName) { $Expected = $Matches[1].ToLower(); break }
        }
    }
    if (-not $Expected) { Fatal "no checksum entry for $AssetName" }

    $Actual = (Get-FileHash -Algorithm SHA256 $ArchivePath).Hash.ToLower()
    if ($Actual -ne $Expected) {
        Fatal "checksum mismatch for $AssetName`n   expected: $Expected`n   actual:   $Actual"
    }
    Info "checksum verified"

    # Ensure install dir exists.
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    Info "extracting..."
    Expand-Archive -Path $ArchivePath -DestinationPath $Tmp -Force
    $Binary = Join-Path $Tmp "dispatch.exe"
    if (-not (Test-Path $Binary)) {
        $Binary = Get-ChildItem -Path $Tmp -Recurse -Filter "dispatch.exe" | Select-Object -First 1 -ExpandProperty FullName
    }
    if (-not (Test-Path $Binary)) { Fatal "dispatch.exe not found in archive" }

    $Target = Join-Path $InstallDir "dispatch.exe"
    Move-Item -Path $Binary -Destination $Target -Force

    Write-Host ""
    Write-Host "  + Dispatch $ReleaseTag installed to $Target" -ForegroundColor Green
    Write-Host ""

    # --- PATH handling ------------------------------------------------------
    $PathUser = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($PathUser -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$PathUser;$InstallDir", "User")
        Info "added $InstallDir to user PATH"
        Info "open a NEW terminal for the PATH change to take effect"
    }
} finally {
    Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
}

Write-Host "  Run: dispatch version"
Write-Host ""
