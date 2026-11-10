#Requires -Version 5.1
<#
.SYNOPSIS
  Install Office Brains (brains) from GitHub Releases, verifying SHA-256.
.EXAMPLE
  irm https://raw.githubusercontent.com/DERYK-RYLOY/office-brains/main/install.ps1 | iex
.NOTES
  Override the release tag with $env:BRAINS_TAG (default: v0.2.0).
  The binary is rejected and not installed if its hash does not match
  SHA256SUMS.txt from the same release.
#>
$ErrorActionPreference = 'Stop'
$Tag = if ($env:BRAINS_TAG) { $env:BRAINS_TAG } else { 'v0.2.0' }
$Repo = 'DERYK-RYLOY/office-brains'
$Asset = 'brains-windows-amd64.exe'
$DestDir = Join-Path $env:USERPROFILE '.office-brains\bin'
$Dest = Join-Path $DestDir 'brains.exe'
$Base = "https://github.com/$Repo/releases/download/$Tag"

New-Item -ItemType Directory -Path $DestDir -Force | Out-Null

Write-Host "Fetching SHA256SUMS.txt for $Tag..."
try {
    $sumsText = (Invoke-WebRequest -Uri "$Base/SHA256SUMS.txt" -UseBasicParsing).Content
} catch {
    Write-Error "could not download SHA256SUMS.txt from $Tag - aborting. $_"
    exit 1
}
$expectedLine = $sumsText -split "`r?`n" | Where-Object { $_ -match [regex]::Escape($Asset) } | Select-Object -First 1
if (-not $expectedLine) {
    Write-Error "SHA256SUMS.txt from $Tag does not list $Asset - aborting."
    exit 1
}
$expected = ($expectedLine -split '\s+')[0].ToLower()

Write-Host "Downloading $Asset..."
Invoke-WebRequest -Uri "$Base/$Asset" -OutFile $Dest

$actual = (Get-FileHash $Dest -Algorithm SHA256).Hash.ToLower()
if ($actual -ne $expected) {
    Remove-Item $Dest -Force
    Write-Error "checksum mismatch: expected $expected, got $actual. The binary was NOT installed."
    exit 1
}
Write-Host "sha256 verified."
& $Dest doctor 2>&1 | Select-Object -First 3
Write-Host "Installed to $Dest. Add $DestDir to PATH to run 'brains' anywhere."
