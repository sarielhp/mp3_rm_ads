#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

if [ $# -eq 0 ]; then
  echo "Usage: $0 <commit-message>"
  exit 1
fi

msg="$*"

bash tools/check.sh > /dev/null 2>&1

git add -A > /dev/null 2>&1
git commit -m "$msg" > /dev/null 2>&1

echo "Success $msg"