#Requires -Version 5.1
<#
.SYNOPSIS
  Install Office Brains (brains) from GitHub Releases.
.EXAMPLE
  irm https://raw.githubusercontent.com/DERYK-RYLOY/office-brains/main/install.ps1 | iex
#>
$ErrorActionPreference = 'Stop'
$Tag = 'v0.1.0'
$Repo = 'DERYK-RYLOY/office-brains'
$Asset = 'brains-windows-amd64.exe'
$DestDir = Join-Path $env:USERPROFILE '.office-brains\bin'
New-Item -ItemType Directory -Path $DestDir -Force | Out-Null
$Dest = Join-Path $DestDir 'brains.exe'
Invoke-WebRequest -Uri "https://github.com/$Repo/releases/download/$Tag/$Asset" -OutFile $Dest
& $Dest doctor 2>&1 | Select-Object -First 3
Write-Host "Installed to $Dest. Add $DestDir to PATH to run 'brains' anywhere."
