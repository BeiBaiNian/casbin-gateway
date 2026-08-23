# Casbin Gateway one-step install for Windows.
#
# Usage (run only if you trust this script source):
#   irm https://raw.githubusercontent.com/apache/casbin-gateway/master/scripts/install.ps1 | iex
#
# This downloads the nightly build, which is an automated build of master and
# not an official release. Use it for testing and development only.
#
# Optional environment variables:
#   INSTALL_DIR   where the executable and its data live
#                 (default: $env:LOCALAPPDATA\casbin-gateway)
#   NO_START      set to any value to install without starting Gateway

$ErrorActionPreference = 'Stop'

$Repo     = 'apache/casbin-gateway'
$Tag      = 'nightly'
$BaseName = 'casbin-gateway-nightly'

$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { "$env:LOCALAPPDATA\casbin-gateway" }
$BinDir     = Join-Path $InstallDir 'bin'

# Windows PowerShell 5.1 still negotiates TLS 1.0 by default, which GitHub
# refuses. The progress bar makes Invoke-WebRequest many times slower on a
# download this size.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$ProgressPreference = 'SilentlyContinue'

function Write-Info { param([string]$Message) Write-Host $Message }

# ── pick the archive for this machine ─────────────────────────────────────────
# Only the x86_64 archive is published; Windows on ARM runs it under the
# built-in x64 emulation.
$ArchName = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'x86_64' }
    'ARM64' {
        Write-Host 'No arm64 build is published, installing the x86_64 one to run under emulation'
        'x86_64'
    }
    default { throw "unsupported architecture `"$env:PROCESSOR_ARCHITECTURE`", build from source instead: https://github.com/$Repo" }
}

$Archive = "$BaseName-windows-$ArchName.zip"
$Url     = "https://github.com/$Repo/releases/download/$Tag/$Archive"

$TmpDir = Join-Path $env:TEMP "casbin-gateway-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TmpDir | Out-Null

try {
    # ── download ──────────────────────────────────────────────────────────────
    $ArchivePath = Join-Path $TmpDir $Archive
    Write-Info "Downloading $Url"
    Invoke-WebRequest -Uri $Url -OutFile $ArchivePath -UseBasicParsing

    Expand-Archive -Path $ArchivePath -DestinationPath $TmpDir -Force
    $Unpacked = Join-Path $TmpDir "$BaseName-windows-$ArchName"

    # ── install ───────────────────────────────────────────────────────────────
    Write-Info "Installing to $InstallDir"
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null

    $ExePath = Join-Path $InstallDir 'casbin-gateway.exe'
    try {
        Copy-Item -Path (Join-Path $Unpacked 'casbin-gateway.exe') -Destination $ExePath -Force
    }
    catch {
        # Windows locks a running executable, so this is what an upgrade over a
        # started Gateway looks like.
        throw "cannot replace $ExePath, stop Casbin Gateway if it is running and try again ($_)"
    }

    foreach ($legalFile in @('LICENSE', 'NOTICE', 'DISCLAIMER')) {
        Copy-Item -Path (Join-Path $Unpacked $legalFile) -Destination (Join-Path $InstallDir $legalFile) -Force
    }
}
finally {
    Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
}

# ── put a "casbin-gateway" command on PATH ────────────────────────────────────
# Gateway keeps its database, logs and temporary files in the working
# directory, so the command is a wrapper that always starts it in $InstallDir.
# Without it, running "casbin-gateway" from somewhere else would quietly start
# a second, empty installation. The wrapper lives in its own directory because
# cmd resolves .exe before .cmd, so a wrapper next to the executable would
# never be the one that runs.
New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
@"
@echo off
rem Written by the Casbin Gateway installer. Gateway reads and writes .\data,
rem .\logs and .\tmp, so it always has to start in its own directory.
cd /d "%~dp0.." || exit /b 1
"%~dp0..\casbin-gateway.exe" %*
"@ | Set-Content -Path (Join-Path $BinDir 'casbin-gateway.cmd') -Encoding ascii

$UserPath = [System.Environment]::GetEnvironmentVariable('PATH', 'User')
if ($UserPath -notlike "*$BinDir*") {
    [System.Environment]::SetEnvironmentVariable('PATH', "$UserPath;$BinDir", 'User')
    $env:PATH = "$env:PATH;$BinDir"
    Write-Info "Added $BinDir to your PATH, which takes effect in your next terminal"
}

# -- start with Windows -------------------------------------------------------
# A shortcut in the Startup folder rather than a service: it needs no elevation,
# shows up in Task Manager's Startup tab, and is undone by deleting the file.
$StartupDir = [System.Environment]::GetFolderPath('Startup')
$StartupLink = Join-Path $StartupDir 'Casbin Gateway.lnk'
if (-not $env:NO_AUTOSTART) {
    try {
        $shell = New-Object -ComObject WScript.Shell
        $shortcut = $shell.CreateShortcut($StartupLink)
        $shortcut.TargetPath = Join-Path $InstallDir 'casbin-gateway.exe'
        $shortcut.Arguments = 'start'
        $shortcut.WorkingDirectory = $InstallDir
        $shortcut.Description = 'Casbin Gateway'
        $shortcut.Save()
        Write-Info 'Casbin Gateway will start with Windows.'
    }
    catch {
        Write-Info "Could not add the startup shortcut: $_"
    }
}

Write-Info ''
Write-Info "Casbin Gateway is installed in $InstallDir"
Write-Info 'Its database, logs and temporary files stay in that directory.'
Write-Info 'It serves this machine only, and signs you in there as admin without a password.'
Write-Info 'Stop it with "casbin-gateway stop", check on it with "casbin-gateway status".'
Write-Info "Remove the startup entry by deleting $StartupLink"
Write-Info ''

if ($env:NO_START) {
    Write-Info 'Start it with: casbin-gateway start'
    return
}

# "start" detaches and returns, so installing does not occupy this terminal.
Set-Location $InstallDir
& (Join-Path $InstallDir 'casbin-gateway.exe') start
