#!/bin/bash
# Rico - Build & Run Script (Linux/macOS)

set -e

echo "=== Rico ==="
echo ""

# 프로젝트 루트로 이동
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/.."

# Frontend 빌드
echo "[1/3] Installing frontend dependencies..."
cd web
npm install --silent
echo "[2/3] Building frontend..."
npm run build
cd ..

# Server 빌드
echo "[3/3] Building server..."
cd server
go mod download
go build -o rico main.go
cd ..

# 실행
echo ""
echo "=== Starting Rico Server ==="
cd server
./rico
