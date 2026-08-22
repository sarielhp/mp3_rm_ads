#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

MAIN_GO="app/constants.go"
if [ ! -f "$MAIN_GO" ]; then
    echo "Error: app/constants.go not found." >&2
    exit 1
fi

CURRENT_VERSION=$(grep -oE '\s*Version\s*=\s*"[0-9]+\.[0-9]+\.[0-9]+"' "$MAIN_GO" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')
if [ -z "$CURRENT_VERSION" ]; then
    echo "Error: Could not parse Version from app/constants.go" >&2
    exit 1
fi

IFS='.' read -r major minor patch <<< "$CURRENT_VERSION"
new_patch=$((patch + 1))
NEW_VERSION="${major}.${minor}.${new_patch}"

sed -i "s/Version\s*=\s*\"$CURRENT_VERSION\"/Version = \"$NEW_VERSION\"/g" "$MAIN_GO"
echo "Version bumped from $CURRENT_VERSION to $NEW_VERSION in app/constants.go"

git add "$MAIN_GO"
git commit -m "bump: version $NEW_VERSION"
echo "Committed: bump: version $NEW_VERSION"

TAG="v$NEW_VERSION"
git tag -a "$TAG" -m "Release $TAG"
echo "Tagged: $TAG"

git push origin HEAD
git push origin "$TAG"
echo "Pushed commit and tag $TAG to origin."