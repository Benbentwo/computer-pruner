#!/bin/bash
set -e

echo "=== ComputerPruner — Project Setup ==="
echo ""

# Check prerequisites
check_cmd() {
  if ! command -v "$1" &> /dev/null; then
    echo "❌ $1 not found."
    return 1
  else
    echo "✅ $1 found: $($1 --version 2>&1 | head -1)"
    return 0
  fi
}

missing=0

echo "Checking prerequisites..."
check_cmd go || missing=1
check_cmd node || missing=1
check_cmd npm || missing=1
check_cmd wails || missing=1

echo ""

if [ $missing -eq 1 ]; then
  echo "Some tools are missing. Install them with:"
  echo ""
  echo "  # Go (if missing)"
  echo "  brew install go"
  echo ""
  echo "  # Node.js (if missing)"
  echo "  brew install node"
  echo ""
  echo "  # Wails CLI (if missing)"
  echo "  go install github.com/wailsapp/wails/v2/cmd/wails@latest"
  echo ""
  echo "Then re-run this script."
  exit 1
fi

echo "All prerequisites found."
echo ""

# Seed the embed placeholder. main.go carries `//go:embed all:frontend/dist`, and an embed
# pattern that matches nothing is a compile error — so on a clean clone every Go command that
# loads the root package fails until frontend/dist holds at least one file. The directory is
# gitignored because it is a build product; `wails build` overwrites this placeholder with the
# real bundle.
echo "=== Seeding frontend/dist placeholder for //go:embed ==="
mkdir -p frontend/dist
if [ ! -f frontend/dist/index.html ]; then
  printf '<!doctype html><html><body></body></html>\n' > frontend/dist/index.html
  echo "Created frontend/dist/index.html"
else
  echo "frontend/dist/index.html already present"
fi
echo ""

# Install frontend dependencies
echo "=== Installing frontend dependencies ==="
cd frontend
npm install
cd ..
echo ""

# Resolve Go dependencies
echo "=== Resolving Go dependencies ==="
go mod tidy
echo ""

# Check Wails environment
echo "=== Checking Wails environment ==="
wails doctor
echo ""

echo "=== ComputerPruner setup complete! ==="
echo ""
echo "Run the app with:"
echo "  wails dev"
echo ""
echo "Build for production with:"
echo "  wails build"
