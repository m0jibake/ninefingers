#!/bin/bash
set -e

# Use nvm to get Node 22 if available
if [ -f ~/.nvm/nvm.sh ]; then
  source ~/.nvm/nvm.sh
  nvm use 22 2>/dev/null || true
fi

echo "Building frontend..."
cd web
npm run build

cd ..
echo "Building Go binary..."
go build -o ninefingers .

echo "✅ Build complete"
