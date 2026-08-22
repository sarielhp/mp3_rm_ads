#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

git add -A
git commit -m "snapshot: $(date '+%Y-%m-%d %H:%M')"
git push origin HEAD
echo "Snapshot pushed."