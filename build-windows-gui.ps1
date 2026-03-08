# C7 CLIENT Windows GUI Build Script
# This PowerShell script builds a Windows GUI executable

param(
    [string]$Architecture = "amd64",
    [Switch]$Build32Bit = $false,
    [Switch]$Portable = $false
)

$ErrorActionPreference = "Stop"

# Colors for output
function Write-Header {
    param([string]$Message)
    Write-Host "`n============================================" -ForegroundColor Cyan
    Write-Host $Message -ForegroundColor Cyan
    Write-Host "============================================`n" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "[✓] $Message" -ForegroundColor Green
}

function Write-Error {
    param([string]$Message)
    Write-Host "[✗] $Message" -ForegroundColor Red
}

function Write-Info {
    param([string]$Message)
    Write-Host "[i] $Message" -ForegroundColor Yellow
}

# Start build process
Write-Header "C7 CLIENT Windows GUI Build"

# Check Go installation
Write-Host "Checking Go installation..."
try {
    $GoVersion = go version
    Write-Success "Go is installed"
    Write-Host $GoVersion
} catch {
    Write-Error "Go is not installed or not in PATH"
    Write-Host "Please install Go from https://golang.org/dl/" -ForegroundColor Yellow
    exit 1
}

# Check for gogio
Write-Host "Checking for gogio build tool..."
$gioCheck = & { gogio -h 2>&1 }
if ($LASTEXITCODE -ne 0) {
    Write-Info "Installing gogio..."
    & go install gioui.org/cmd/gogio@latest
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Failed to install gogio"
        exit 1
    }
}
Write-Success "gogio is available"

# Create builds directory
if (-not (Test-Path "builds")) {
    New-Item -ItemType Directory -Path "builds" | Out-Null
}
Write-Success "builds directory ready"

# Get version information
try {
    $BuildTag = & git describe --tags --always --match "v*" 2>$null
    if (-not $BuildTag) {
        $BuildTag = "dev"
        Write-Info "No git tags found, using dev version"
    } else {
        Write-Success "Git tag: $BuildTag"
    }
} catch {
    $BuildTag = "dev"
    Write-Info "Git not available, using dev version"
}

# Build function
function Build-GUI {
    param(
        [string]$Arch,
        [string]$OutputFile
    )
    
    Write-Host "Building C7 CLIENT GUI for Windows ($Arch)..."
    Write-Host "Command: gogio -arch $Arch -target windows -version 1.0.0 -icon icon.png -o `"$OutputFile`" ./cmd/bedrocktool`n"
    
    & gogio -arch $Arch -target windows -version 1.0.0 -icon icon.png -o $OutputFile ./cmd/bedrocktool
    
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Build successful: $OutputFile"
        return $true
    } else {
        Write-Error "Build failed"
        return $false
    }
}

# Main build
Write-Host "Building C7 CLIENT GUI for Windows ($Architecture)...`n"
$Success = Build-GUI $Architecture "builds/c7client-gui-windows-$Architecture.exe"

if (-not $Success) {
    exit 1
}

# Optional 32-bit build
if ($Build32Bit) {
    Write-Host "`nBuilding 32-bit version...`n"
    $Success32 = Build-GUI "386" "builds/c7client-gui-windows-386.exe"
    if (-not $Success32) {
        Write-Error "32-bit build failed (continuing anyway)"
    }
}

# Build summary
Write-Header "Build Complete!"

Write-Host "Output Files:`n"
Get-ChildItem "builds/c7client-gui-windows-*.exe" -ErrorAction SilentlyContinue | ForEach-Object {
    $Size = [math]::Round($_.Length / 1MB, 2)
    Write-Host "  • $($_.Name) ($Size MB)" -ForegroundColor Green
}

Write-Host "`nBuild Information:`n"
Write-Host "  • Platform: Windows"
Write-Host "  • Version: $BuildTag"
Write-Host "  • Type: GUI Application"
Write-Host "  • Framework: Gio (gioui.org)"

Write-Host "`nAdditional Features:`n"
Write-Host "  • Modular utility framework (C7 CLIENT)"
Write-Host "  • Player tracking module"
Write-Host "  • Real-time position updates"
Write-Host "  • Distance and direction display"

Write-Host "`nNext Steps:`n"
Write-Host "  1. Run the executable: .\builds\c7client-gui-windows-amd64.exe" -ForegroundColor Cyan
Write-Host "  2. Select a utility feature from the menu" -ForegroundColor Cyan
Write-Host "  3. Configure server connection" -ForegroundColor Cyan
Write-Host "  4. Connect and enjoy!" -ForegroundColor Cyan

Write-Host "`n"
Write-Success "Build process complete!"
