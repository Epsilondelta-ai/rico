#!/bin/bash
# Rico - Quick Restart Script (Linux/macOS)
# Frontend: dev server (no build), Backend: build and run

cd "$(dirname "$0")/.."

echo "=== Rico Quick Restart ==="
echo

# Kill existing processes on ports
echo "Stopping existing servers..."
npx kill-port 5173 8081 2>/dev/null || true

# Wait a moment
sleep 2

# Start frontend dev server in background (no build)
echo "Starting frontend (npm run dev)..."
cd web
npm run dev &
cd ..

# Build and start backend server
echo "Building backend server..."
cd server
go build -o server
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi
echo "Starting backend server..."
./server
