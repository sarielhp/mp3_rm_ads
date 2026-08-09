#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "=== Formatting ==="
gofmt -s -w .

echo "=== Tidy ==="
go mod tidy

echo "=== Vet ==="
go vet ./...

echo "=== Staticcheck ==="
staticcheck -checks '-SA2001' ./...

echo "=== Test ==="
go test -timeout 30s ./...

echo "=== Build ==="
go build -o mp3_rm_ads .

echo ""
echo "All checks passed."