#!/usr/bin/env bash
set -euo pipefail

MODULES=()

# Install golangci-lint
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.2.1

# Get all modules
while IFS= read -r line; do
  MODULES+=("$line")
done < <(go list -f '{{.Dir}}' -m)

# Append '/...' to each module path
for i in "${!MODULES[@]}"; do
  MODULES[$i]="${MODULES[$i]}/..."
done

exec golangci-lint run "${MODULES[@]}" --timeout=5m "$@"
