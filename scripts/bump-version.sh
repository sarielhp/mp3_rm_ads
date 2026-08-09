#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

current=$(cat VERSION)
IFS='.' read -r major minor patch <<< "$current"
patch=$((patch + 1))
new="$major.$minor.$patch"
echo "$new" > VERSION
echo "Bumped: $current -> $new"