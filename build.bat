@echo off
setlocal

set "BIN_DIR=%~dp0bin"
if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"

go build -o "%BIN_DIR%\nolvegen.exe"
if errorlevel 1 exit /b 1

echo Built %BIN_DIR%\nolvegen.exe
