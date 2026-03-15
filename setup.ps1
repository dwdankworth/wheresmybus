#Requires -Version 5.1
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ---------- colors ----------
function Info  { param([string]$msg) Write-Host "i  $msg" -ForegroundColor Blue }
function Ok    { param([string]$msg) Write-Host "+  $msg" -ForegroundColor Green }
function Warn  { param([string]$msg) Write-Host "!  $msg" -ForegroundColor Yellow }
function Fail  { param([string]$msg) Write-Host "x  $msg" -ForegroundColor Red; exit 1 }

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# ---------- 1. Check Go is installed ----------
Info 'Checking for Go...'
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Fail 'Go is not installed. Install it from https://go.dev/dl/ and re-run this script.'
}
$goVersionOutput = & go version
Ok "Go found: $goVersionOutput"

# ---------- 2. Enforce minimum Go version ----------
$requiredMajor = 1
$requiredMinor = 25
$requiredPatch = 8

if ($goVersionOutput -match 'go(\d+)\.(\d+)(?:\.(\d+))?') {
    $gotMajor = [int]$Matches[1]
    $gotMinor = [int]$Matches[2]
    $gotPatch = if ($Matches[3]) { [int]$Matches[3] } else { 0 }
} else {
    Fail 'Could not parse Go version.'
}

$versionOk = $false
if ($gotMajor -gt $requiredMajor) {
    $versionOk = $true
} elseif ($gotMajor -eq $requiredMajor) {
    if ($gotMinor -gt $requiredMinor) {
        $versionOk = $true
    } elseif ($gotMinor -eq $requiredMinor -and $gotPatch -ge $requiredPatch) {
        $versionOk = $true
    }
}

if (-not $versionOk) {
    Fail "Go ${requiredMajor}.${requiredMinor}.${requiredPatch}+ is required (found ${gotMajor}.${gotMinor}.${gotPatch}). Please upgrade: https://go.dev/dl/"
}
Ok "Go version ${gotMajor}.${gotMinor}.${gotPatch} meets minimum requirement (${requiredMajor}.${requiredMinor}.${requiredPatch})"

# ---------- 3. Build the binary ----------
Info 'Building wheresmybus...'
Push-Location $ScriptDir
try {
    & go build -o wheresmybus.exe .
    if ($LASTEXITCODE -ne 0) { Fail 'Build failed.' }
} finally {
    Pop-Location
}
Ok 'Built .\wheresmybus.exe'

# ---------- 4. Offer to install to PATH ----------
$InstallDir = Join-Path $env:LOCALAPPDATA 'wheresmybus'

Write-Host ''
Write-Host 'Add wheresmybus to your PATH?' -NoNewline -ForegroundColor White
Write-Host ''
Write-Host "  This copies the binary to $InstallDir and adds it to your user PATH."

$answer = Read-Host '  Install to PATH? [Y/n]'
if ([string]::IsNullOrWhiteSpace($answer)) { $answer = 'Y' }

if ($answer -match '^[Yy]') {
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    $targetExe = Join-Path $InstallDir 'wheresmybus.exe'
    if (Test-Path $targetExe) {
        $overwrite = Read-Host "  $targetExe already exists. Overwrite? [Y/n]"
        if ([string]::IsNullOrWhiteSpace($overwrite)) { $overwrite = 'Y' }
        if ($overwrite -notmatch '^[Yy]') {
            Warn 'Skipped PATH installation.'
        } else {
            Copy-Item (Join-Path $ScriptDir 'wheresmybus.exe') $targetExe -Force
            Ok "Updated $targetExe"
        }
    } else {
        Copy-Item (Join-Path $ScriptDir 'wheresmybus.exe') $targetExe -Force
        Ok "Installed to $targetExe"
    }

    # Add to user PATH if not already present
    $userPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
    if ($userPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable('PATH', "$InstallDir;$userPath", 'User')
        $env:PATH = "$InstallDir;$env:PATH"
        Ok "Added $InstallDir to user PATH"
        Warn 'Open a new terminal for PATH changes to take effect.'
    } else {
        Ok "$InstallDir is already on PATH"
    }
} else {
    Info 'Skipped PATH installation. You can run .\wheresmybus.exe from this directory.'
}

# ---------- 5. Configure .env ----------
Write-Host ''
Write-Host 'Configure .env' -ForegroundColor White

$ConfigDir = & (Join-Path $ScriptDir 'wheresmybus.exe') --print-config-dir
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($ConfigDir)) {
    Fail 'Could not determine the config directory.'
}
$envFile = Join-Path $ConfigDir '.env'
$legacyEnvFile = Join-Path $ScriptDir '.env'

if (Test-Path $envFile) {
    $reconfig = Read-Host '  .env already exists. Reconfigure? [y/N]'
    if ([string]::IsNullOrWhiteSpace($reconfig)) { $reconfig = 'N' }
    if ($reconfig -notmatch '^[Yy]') {
        Ok "Keeping existing $envFile"
        Write-Host ''
        Write-Host 'Setup complete!' -ForegroundColor Green
        Write-Host "Config stored at $envFile"
        Write-Host 'Run ' -NoNewline; Write-Host 'wheresmybus' -ForegroundColor White -NoNewline; Write-Host ' to see your next bus.'
        exit 0
    }
}

if (Test-Path $legacyEnvFile) {
    if (-not (Test-Path $ConfigDir)) {
        New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
    }
    Copy-Item $legacyEnvFile $envFile -Force
    Ok "Copied existing .env from $legacyEnvFile to $envFile"
    Write-Host ''
    Write-Host 'Setup complete!' -ForegroundColor Green
    Write-Host "Config stored at $envFile"
    Write-Host 'Run ' -NoNewline; Write-Host 'wheresmybus' -ForegroundColor White -NoNewline; Write-Host ' to see your next bus.'
    exit 0
}

Info "Let's configure your settings."
Write-Host ''

# API key
Write-Host '  OneBusAway API Key' -ForegroundColor White
Write-Host '  Sign up at https://www.pugetsound.onebusaway.org/p/sign-up' -ForegroundColor Blue
$obaApiKey = Read-Host '  API key'
if ([string]::IsNullOrWhiteSpace($obaApiKey)) { Fail 'API key is required.' }

# Home wifi
Write-Host ''
Write-Host '  Home WiFi network name (used for auto-detecting direction)' -ForegroundColor White
$homeWifi = Read-Host '  Home WiFi SSID'

# Office wifi
Write-Host ''
Write-Host '  Office WiFi network name' -ForegroundColor White
$officeWifi = Read-Host '  Office WiFi SSID'

# Stop IDs
Write-Host ''
Write-Host '  Bus stop IDs' -ForegroundColor White
Write-Host '  Find yours at https://pugetsound.onebusaway.org (e.g. 1_75403)' -ForegroundColor Blue
$homeStopId = Read-Host '  Home stop ID'
$officeStopId = Read-Host '  Office stop ID'

if (-not (Test-Path $ConfigDir)) {
    New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
}

@"
OBA_API_KEY=$obaApiKey
HOME_WIFI=$homeWifi
OFFICE_WIFI=$officeWifi
HOME_STOP_ID=$homeStopId
OFFICE_STOP_ID=$officeStopId
"@ | Set-Content -Path $envFile -Encoding UTF8 -NoNewline

Ok "Wrote $envFile"

# ---------- Done ----------
Write-Host ''
Write-Host 'Setup complete!' -ForegroundColor Green
Write-Host "Config stored at $envFile"
Write-Host 'Run ' -NoNewline; Write-Host 'wheresmybus' -ForegroundColor White -NoNewline; Write-Host ' to see your next bus.'
