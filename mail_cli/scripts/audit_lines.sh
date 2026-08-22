#!/usr/bin/env bash
set -euo pipefail

# Ensure we are in the repository root
cd "$(dirname "$0")/.."

echo "Auditing Go source files line counts..."
echo "----------------------------------------"

EXCEED=0
for f in *.go; do
    if [ -f "$f" ]; then
        line_count=$(wc -l < "$f")
        if [ "$line_count" -gt 500 ]; then
            echo -e "\e[31m[!] $f: $line_count lines (exceeds 500-line soft limit)\e[0m"
            EXCEED=$((EXCEED + 1))
        else
            echo "    $f: $line_count lines"
        fi
    fi
done

echo "----------------------------------------"
if [ "$EXCEED" -gt 0 ]; then
    echo "Audit finished: $EXCEED file(s) exceed the 500-line soft limit."
else
    echo "Audit finished: All files are within the 500-line soft limit."
fi
