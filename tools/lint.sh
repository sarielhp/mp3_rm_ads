#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== Vet ==="
go vet ./...

echo "=== Staticcheck ==="
staticcheck -checks '-SA2001' ./...

echo "=== Line Audit ==="
ruby tools/audit_lines.rb