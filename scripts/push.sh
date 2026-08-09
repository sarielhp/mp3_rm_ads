#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

git push

bash scripts/bump-version.sh

echo ""
echo "Pushed and bumped version to $(cat VERSION)"