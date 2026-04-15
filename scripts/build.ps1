# Build script for stand-reminder with version injection
# Usage: 
#   ./scripts/build.ps1                    # Use version from version.go
#   ./scripts/build.ps1 -Version "v0.6.0"  # Inject specific version

param(
    [string]$Version = "",
    [string]$OutputDir = "dist"
)

$repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)

# Read current version from version.go if not provided
if (-not $Version) {
    $versionFile = Join-Path $repoRoot "version.go"
    $versionContent = Get-Content $versionFile
    if ($versionContent -match 'var Version = "([^"]+)"') {
        $Version = $matches[1]
    } else {
        Write-Error "Failed to read version from version.go"
        exit 1
    }
}

Write-Host "Building stand-reminder v$Version..." -ForegroundColor Green

# Create output directory
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

# Build with ldflags to inject version
$ldflags = "-X main.Version=$Version"
$cmd = "go build -ldflags `"$ldflags`" -o `"$(Join-Path $OutputDir 'stand-reminder.exe')`" ."

Write-Host "Command: $cmd"
Invoke-Expression $cmd

if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Build successful: $(Join-Path $OutputDir 'stand-reminder.exe')" -ForegroundColor Green
} else {
    Write-Error "Build failed"
    exit 1
}
