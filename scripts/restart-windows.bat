@echo off
REM Rico - Quick Restart Script (Windows)
REM Frontend: dev server (no build), Backend: build and run

REM Get the absolute root directory (parent of scripts)
cd /d "%~dp0.."
set "ROOT_DIR=%CD%"
set "LOG_FILE=%ROOT_DIR%\logs\restart.log"

echo === Rico Quick Restart === > "%LOG_FILE%"
echo [%date% %time%] Starting restart... >> "%LOG_FILE%"
echo ROOT_DIR: %ROOT_DIR% >> "%LOG_FILE%"

echo === Rico Quick Restart ===
echo ROOT_DIR: %ROOT_DIR%
echo.

REM Kill existing servers on ports 5173 and 8081
echo Stopping existing servers...
cd /d "%ROOT_DIR%\web"
call npx kill-port 5173 8081
echo Ports cleared!
echo.

REM Build backend server
echo Building backend server...
cd /d "%ROOT_DIR%\server"
go build -o server.exe
if %ERRORLEVEL% NEQ 0 (
    echo Build failed!
    pause
    exit /b 1
)
echo Build complete!

REM Start backend server in new window
echo Starting backend server...
start "Rico Backend" cmd /k "cd /d %ROOT_DIR%\server && .\server.exe"

REM Wait a moment
timeout /t 2 >nul

REM Start frontend dev server in new window
echo Starting frontend (npm run dev)...
start "Rico Frontend" cmd /k "cd /d %ROOT_DIR%\web && npm run dev"

echo.
echo Rico servers started!
echo [%date% %time%] Restart complete! >> "%LOG_FILE%"
echo This window will close in 3 seconds...
timeout /t 3 >nul
