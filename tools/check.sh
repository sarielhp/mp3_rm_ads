#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "=== Formatting & Config Template ==="
./tools/format.sh

echo "=== Tidy ==="
go mod tidy

echo "=== Lint & Line Audit ==="
./tools/lint.sh

echo "=== Test ==="
go test -timeout 30s ./...

echo "=== Build ==="
go build -o abs .

echo ""
echo "All checks passed."