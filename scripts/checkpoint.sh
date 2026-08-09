#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ -n $(git status --porcelain) ]]; then
    git add -A
    git commit -m "wip: checkpoint [$(date +'%H:%M:%S')]" --no-verify
    echo "Checkpoint saved."
else
    echo "Nothing to checkpoint."
fi