#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

if [ $# -eq 0 ]; then
  echo "Usage: $0 <commit-message>"
  exit 1
fi

msg="$*"

bash scripts/check.sh

git add -A
git commit -m "$msg"

echo ""
echo "Committed: $msg"