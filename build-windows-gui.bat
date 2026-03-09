@echo off
REM C7 CLIENT Windows GUI Build Script
REM This script builds a Windows GUI executable

setlocal enabledelayedexpansion

REM Set up colors for output (using ANSI codes - works on Windows 10+)
for /F %%A in ('echo prompt $H ^| cmd') do set "BS=%%A"

echo.
echo ============================================
echo C7 CLIENT Windows GUI Build
echo ============================================
echo.

REM Check Go installation
go version >nul 2>&1
if errorlevel 1 (
    echo Error: Go is not installed or not in PATH
    echo Please install Go from https://golang.org/dl/
    exit /b 1
)

echo [OK] Go is installed
go version

REM Get Go version
for /f "tokens=3" %%i in ('go version') do set GOVERSION=%%i
echo Go Version: !GOVERSION!
echo.

REM Check for Gio build tool
echo Checking for gogio build tool...
where gogio >nul 2>&1
if errorlevel 1 (
    echo.
    echo Installing gogio...
    go install gioui.org/cmd/gogio@latest
    if errorlevel 1 (
        echo Error: Failed to install gogio
        exit /b 1
    )
)

echo [OK] gogio is available
gogio -h >nul 2>&1

REM Create builds directory
if not exist "builds" mkdir builds
echo [OK] builds directory ready

REM Get version information
for /f "delims=" %%i in ('git describe --tags --always --match "v*"') do set BUILDTAG=%%i
if "!BUILDTAG!"=="" (
    set BUILDTAG=dev
    echo [!] No git tags found, using dev version
) else (
    echo [OK] Git tag: !BUILDTAG!
)

REM Build GUI for Windows (64-bit)
echo.
echo Building C7 CLIENT GUI for Windows (64-bit)...
echo Command: gogio -arch amd64 -target windows -tags gui -version 1.0.0 -icon icon.png -o "builds/C7 Proxy Client.exe" ./cmd/bedrocktool
echo.

gogio -arch amd64 -target windows -tags gui -version 1.0.0 -icon icon.png -o "builds/C7 Proxy Client.exe" ./cmd/bedrocktool
if errorlevel 1 (
    echo Error: Build failed
    exit /b 1
)

echo.
echo ============================================
echo [SUCCESS] Build Complete!
echo ============================================
echo.
echo Output: builds/C7 Proxy Client.exe
echo.
echo Build Information:
echo   - Platform: Windows 64-bit
echo   - Version: !BUILDTAG!
echo   - Type: GUI Application
echo.

REM Optional: Build 32-bit version
set /p BUILD32="Build 32-bit version as well? (y/n): "
if /i "!BUILD32!"=="y" (
    echo.
    echo Building C7 CLIENT GUI for Windows (32-bit)...
    gogio -arch 386 -target windows -tags gui -version 1.0.0 -icon icon.png -o "builds/c7client-gui-windows-386.exe" ./cmd/bedrocktool
    if errorlevel 1 (
        echo Warning: 32-bit build failed
    ) else (
        echo [OK] 32-bit build complete: builds/c7client-gui-windows-386.exe
    )
)

echo.
echo Your executable is ready to use!
echo.
pause
