#!/usr/bin/env bash
set -euo pipefail

go_binary="${GO:-go}"
packages=()
while IFS= read -r package; do
  if [[ "$package" != */internal/game ]]; then
    packages+=("$package")
  fi
done < <("$go_binary" list ./internal/...)

exec "$go_binary" test "${packages[@]}" "$@"
