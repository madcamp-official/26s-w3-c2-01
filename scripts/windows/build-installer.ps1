<#
.SYNOPSIS
  Builds libra.exe and packages it into a Windows installer (libra-setup-<version>.exe)
  using Inno Setup.

.DESCRIPTION
  Requires Inno Setup 6 (ISCC.exe) to be installed:
    winget install JRSoftware.InnoSetup

.PARAMETER Version
  Version string to embed in the installer (defaults to `git describe --tags`,
  falling back to "0.0.0-dev" if no tags exist).
#>
param(
    [string]$Version
)

$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Push-Location $RepoRoot
try {
    if (-not $Version) {
        try {
            $Version = (git describe --tags --always 2>$null).Trim()
        } catch {
            $Version = ""
        }
        if (-not $Version) {
            $Version = "0.0.0-dev"
        }
    }

    Write-Host "Building libra.exe (installer version $Version)..."
    $ldflags = "-s -w -X github.com/madcamp-official/26s-w3-c2-01/cmd.Version=$Version"
    go build -ldflags $ldflags -o libra.exe .

    $isccCmd = Get-Command ISCC.exe -ErrorAction SilentlyContinue
    if ($isccCmd) {
        $isccPath = $isccCmd.Source
    } else {
        $candidates = @(
            "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
            "$env:ProgramFiles\Inno Setup 6\ISCC.exe",
            "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe"
        )
        $isccPath = $candidates | Where-Object { Test-Path $_ } | Select-Object -First 1
    }
    if (-not $isccPath) {
        throw "ISCC.exe not found. Install Inno Setup 6 first: winget install JRSoftware.InnoSetup"
    }

    New-Item -ItemType Directory -Force -Path (Join-Path $RepoRoot "dist") | Out-Null

    Write-Host "Compiling installer..."
    & $isccPath "/DMyAppVersion=$Version" "scripts\windows\libra.iss"

    Write-Host "Done: dist\libra-setup-$Version.exe"
} finally {
    Pop-Location
}
