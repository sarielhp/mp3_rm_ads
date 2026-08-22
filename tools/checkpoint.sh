#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ -n $(git status --porcelain) ]]; then
    git add -A > /dev/null 2>&1
    git commit -m "wip: checkpoint [$(date +'%H:%M:%S')]" --no-verify > /dev/null 2>&1
    echo "Checkpoint saved."
fi