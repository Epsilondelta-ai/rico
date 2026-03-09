@echo off
REM Rico - Build & Run Script (Windows)
REM Frontend: production build, Backend: build and run

REM Get the absolute root directory (parent of scripts)
cd /d "%~dp0.."
set "ROOT_DIR=%CD%"

echo === Rico Build ^& Run ===
echo ROOT_DIR: %ROOT_DIR%
echo.

REM Kill existing servers on ports 5173 and 8081
echo Stopping existing servers...
cd /d "%ROOT_DIR%\web"
call npx kill-port 5173 8081
echo Ports cleared!
echo.

REM Frontend 빌드
echo [1/3] Installing frontend dependencies...
cd /d "%ROOT_DIR%\web"
call npm install --silent
echo [2/3] Building frontend...
call npm run build
if %ERRORLEVEL% NEQ 0 (
    echo Frontend build failed!
    pause
    exit /b 1
)

REM Server 빌드
echo [3/3] Building server...
cd /d "%ROOT_DIR%\server"
go mod download
go build -o server.exe main.go
if %ERRORLEVEL% NEQ 0 (
    echo Server build failed!
    pause
    exit /b 1
)
echo Build complete!

REM Start backend server in new window
echo.
echo === Starting Rico Server ===
start "Rico Backend" cmd /k "cd /d %ROOT_DIR%\server && .\server.exe"

echo.
echo Rico server started in a new window!
echo This window will close in 3 seconds...
timeout /t 3 >nul
