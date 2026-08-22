#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
gofmt -s -w .
echo "Formatted."