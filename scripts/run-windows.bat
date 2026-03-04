@echo off
REM Rico - Build & Run Script (Windows)

echo === Rico ===
echo.

REM 프로젝트 루트로 이동
cd /d "%~dp0.."

REM Frontend 빌드
echo [1/3] Installing frontend dependencies...
cd web
call npm install --silent
echo [2/3] Building frontend...
call npm run build
cd ..

REM Server 빌드
echo [3/3] Building server...
cd server
go mod download
go build -o rico.exe main.go
cd ..

REM 실행
echo.
echo === Starting Rico Server ===
cd server
rico.exe
