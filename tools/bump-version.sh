#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

current=$(cat VERSION)
IFS='.' read -r major minor patch <<< "$current"
patch=$((patch + 1))
new="$major.$minor.$patch"
echo "$new" > VERSION

git add VERSION > /dev/null 2>&1
git commit -m "chore: bump version to $new" > /dev/null 2>&1
git push > /dev/null 2>&1

echo "Success $new (commit+push)"