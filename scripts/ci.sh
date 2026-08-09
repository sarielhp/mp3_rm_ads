#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== Full CI Pipeline ==="
echo "Version: $(cat VERSION)"
echo ""

bash scripts/check.sh

echo "=== CI complete ==="