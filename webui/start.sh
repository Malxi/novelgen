#!/bin/bash

echo "🚀 Starting NovelGen Web UI..."

# Build frontend
echo "📦 Building frontend..."
cd "$(dirname "$0")/frontend"
npm run build
if [ $? -ne 0 ]; then
    echo "❌ Frontend build failed"
    exit 1
fi

# Start backend
echo "🔧 Starting backend server..."
cd "$(dirname "$0")"
go run main.go
