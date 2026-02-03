@echo off
echo ========================================
echo Building Desktop Application...
echo ========================================

cd /d "%~dp0..\src\MailOpsDesktop"

REM Restore NuGet packages
echo Restoring NuGet packages...
dotnet restore

if %ERRORLEVEL% NEQ 0 (
    echo ERROR: dotnet restore failed!
    pause
    exit /b 1
)

REM Build the project
echo Building MailOpsDesktop.exe...
dotnet build -c Release -p:PublishSingleFile=true -p:SelfContained=true -p:RuntimeIdentifier=win-x64

if %ERRORLEVEL% NEQ 0 (
    echo ERROR: dotnet build failed!
    pause
    exit /b 1
)

REM Copy output to dist directory
echo Copying files to dist...
if not exist "%~dp0..\dist" mkdir "%~dp0..\dist"
copy /Y "bin\Release\net6.0-windows\win-x64\publish\MailOpsDesktop.exe" "%~dp0..\dist"
copy /Y "bin\Release\net6.0-windows\win-x64\publish\MailOpsDesktop.dll" "%~dp0..\dist" 2>nul

echo SUCCESS: MailOpsDesktop.exe built successfully!
echo Output: dist\MailOpsDesktop.exe
echo.

pause
