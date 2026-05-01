param(
    [string]$Version = "0.6.3"
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir
$ExePath = Join-Path $ProjectRoot "stand-reminder.exe"
$IssPath = Join-Path $ProjectRoot "setup.iss"
$OutputDir = Join-Path $ProjectRoot "dist"

# Build Go binary
Write-Host "Building stand-reminder.exe..." -ForegroundColor Green
go build -ldflags="-s -w" -o $ExePath .

# Create dist directory
New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null

# Compile Inno Setup installer
$iscc = Get-Command "ISCC.exe" -ErrorAction SilentlyContinue
if (-not $iscc) {
    $isccPath = "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe"
    if (-not (Test-Path $isccPath)) {
        $isccPath = "${env:LOCALAPPDATA}\Programs\Inno Setup 6\ISCC.exe"
    }
    if (-not (Test-Path $isccPath)) {
        Write-Error "Inno Setup not found. Install from https://jrsoftware.org/isdl.php"
        exit 1
    }
    $iscc = $isccPath
} else {
    $iscc = $iscc.Source
}

Write-Host "Compiling installer..." -ForegroundColor Green
& $iscc "/O$OutputDir" "/DMyAppVersion=$Version" $IssPath

Write-Host "Done! Installer at: $OutputDir" -ForegroundColor Green
