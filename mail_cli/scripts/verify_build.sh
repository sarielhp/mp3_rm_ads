#!/usr/bin/env bash
set -euo pipefail

# Ensure we are in the repository root
cd "$(dirname "$0")/.."

echo "1. Formatting code..."
go fmt ./...

echo "2. Running tests..."
go test -v ./...

echo "3. Compiling binary..."
make

echo "Verification completed successfully!"
