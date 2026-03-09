#!/bin/bash
# Rico - Build & Run Script (Linux/macOS)
# Frontend: production build, Backend: build and run

set -e

# Get the absolute root directory (parent of scripts)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

echo "=== Rico Build & Run ==="
echo "ROOT_DIR: $ROOT_DIR"
echo

# Kill existing processes on ports
echo "Stopping existing servers..."
npx kill-port 5173 8081 2>/dev/null || true
echo "Ports cleared!"
echo

# Frontend 빌드
echo "[1/3] Installing frontend dependencies..."
cd "$ROOT_DIR/web"
npm install --silent
echo "[2/3] Building frontend..."
npm run build

# Server 빌드
echo "[3/3] Building server..."
cd "$ROOT_DIR/server"
go mod download
go build -o server main.go
echo "Build complete!"

# Start backend server in background
echo
echo "=== Starting Rico Server ==="
cd "$ROOT_DIR/server"
./server &

echo
echo "Rico server started in background (PID: $!)!"
