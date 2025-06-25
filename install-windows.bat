@echo off
REM Easy SSH Tunnel Manager Windows Installer
REM Requires PowerShell and curl/wget

setlocal enabledelayedexpansion

echo.
echo 🚇 Easy SSH Tunnel Manager - Windows Installer
echo ============================================
echo.

REM Configuration
set REPO_OWNER=ivikasavnish
set REPO_NAME=easytunnel
set BINARY_NAME=easytunnel
set INSTALL_DIR=%ProgramFiles%\EasyTunnel

REM Check for admin privileges
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo ERROR: Administrator privileges required
    echo Please run this script as Administrator
    pause
    exit /b 1
)

REM Get latest version using PowerShell
echo Fetching latest release information...
for /f "delims=" %%i in ('powershell -command "(Invoke-RestMethod https://api.github.com/repos/%REPO_OWNER%/%REPO_NAME%/releases/latest).tag_name"') do set VERSION=%%i

if "%VERSION%"=="" (
    echo ERROR: Could not fetch latest version
    pause
    exit /b 1
)

echo Latest version: %VERSION%

REM Create installation directory
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"

REM Download and extract
set DOWNLOAD_URL=https://github.com/%REPO_OWNER%/%REPO_NAME%/releases/download/%VERSION%/%BINARY_NAME%-%VERSION%-windows-amd64.zip
set TEMP_FILE=%TEMP%\%BINARY_NAME%-%VERSION%-windows-amd64.zip

echo Downloading %DOWNLOAD_URL%
powershell -command "Invoke-WebRequest -Uri '%DOWNLOAD_URL%' -OutFile '%TEMP_FILE%'"

if not exist "%TEMP_FILE%" (
    echo ERROR: Download failed
    pause
    exit /b 1
)

echo Extracting to %INSTALL_DIR%...
powershell -command "Expand-Archive -Path '%TEMP_FILE%' -DestinationPath '%INSTALL_DIR%' -Force"

REM Add to PATH
echo Adding to system PATH...
powershell -command "[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'Machine') + ';%INSTALL_DIR%', 'Machine')"

REM Cleanup
del "%TEMP_FILE%"

echo.
echo ✅ Installation complete!
echo.
echo Quick Start:
echo   %BINARY_NAME%                    - Start the application
echo   %BINARY_NAME% --help             - Show help
echo.
echo Web Interface:
echo   Open http://localhost:10000 in your browser
echo.
echo The application has been installed to: %INSTALL_DIR%
echo You may need to restart your command prompt for PATH changes to take effect.
echo.
pause
