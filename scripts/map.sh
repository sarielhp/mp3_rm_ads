#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== Package Structure ==="
find . -maxdepth 3 -not -path '*/.*' -not -path '*/vendor/*' -type d | sort

echo ""
echo "=== Key Structs & Interfaces ==="
grep -rnE "^type [A-Z]\w+ (struct|interface)" --include="*.go" . | grep -v "_test.go" | sort || echo "(none)"

echo ""
echo "=== Top-Level Functions ==="
grep -rnE "^func \w+" --include="*.go" . | grep -v "_test.go" | sed 's/^[^:]*:[^:]*://' | sort || echo "(none)"

echo ""
echo "=== Exported Functions ==="
grep -rnE "^func [A-Z]\w+" --include="*.go" . | grep -v "_test.go" | sed 's/^[^:]*:[^:]*://' | sort || echo "(none)"