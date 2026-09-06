#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
gofmt -s -w .
./tools/generate_config_template > /dev/null 2>&1
echo "Formatted and synced config template."