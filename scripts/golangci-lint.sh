#!/usr/bin/env bash
set -euo pipefail

MODULES=()

# Get all modules
while IFS= read -r line; do
  MODULES+=("$line")
done < <(go list -f '{{.Dir}}' -m)

# Append '/...' to each module path
for i in "${!MODULES[@]}"; do
  MODULES[$i]="${MODULES[$i]}/..."
done

exec golangci-lint run "${MODULES[@]}" --timeout=5m "$@"
