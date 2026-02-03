@echo off
echo ========================================
echo Checking Go Service...
echo ========================================

cd /d "%~dp0"

REM Check if service.exe exists in dist
if exist "dist\mailops-service.exe" (
    echo SUCCESS: mailops-service.exe found in dist/
    echo.
    goto :end
)

REM Check if service.exe exists in workspace
if exist "..\..\dist\mailops-service.exe" (
    echo Found mailops-service.exe in ../../dist/
    echo Copying to dist/...
    
    if not exist "dist" mkdir "dist"
    copy "..\..\dist\mailops-service.exe" "dist\mailops-service.exe"
    
    if %ERRORLEVEL% NEQ 0 (
        echo ERROR: Failed to copy mailops-service.exe
        pause
        exit /b 1
    )
    
    echo SUCCESS: mailops-service.exe copied to dist/
    echo.
    goto :end
)

echo ERROR: mailops-service.exe not found!
echo.
echo Please ensure mailops-service.exe exists in one of these locations:
echo   1. dist/mailops-service.exe
echo   2. ../../dist/mailops-service.exe
echo.
echo You can build it by running: go build -o dist/mailops-service.exe ./cmd/service
echo (from the root workspace directory)
echo.
pause
exit /b 1

:end
echo Build script completed.
pause