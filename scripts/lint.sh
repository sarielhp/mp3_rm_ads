#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== Vet ==="
go vet ./...

echo "=== Staticcheck ==="
staticcheck -checks '-SA2001' ./...