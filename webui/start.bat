@echo off
chcp 65001 >nul
echo 🚀 Starting NovelGen Web UI...

:: Build frontend
echo 📦 Building frontend...
cd /d "%~dp0frontend"
call npm run build
if errorlevel 1 (
    echo ❌ Frontend build failed
    pause
    exit /b 1
)

:: Start backend
echo 🔧 Starting backend server...
cd /d "%~dp0"
go run main.go

pause
