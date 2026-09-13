#!/usr/bin/env bash
set -euo pipefail

if [[ $# -eq 0 ]]; then
  set -- ./...
fi

go_binary="${GO:-go}"

if [[ "$(uname -s)" == "Linux" && -z "${DISPLAY:-}" ]]; then
  if ! command -v xvfb-run >/dev/null 2>&1; then
    echo "error: DISPLAY is unset and xvfb-run is unavailable" >&2
    echo "install Xvfb or run the tests from an active graphical session" >&2
    exit 1
  fi
  exec xvfb-run -a -s "-screen 0 ${XVFB_SCREEN:-1024x768x24}" "$go_binary" test "$@"
fi

exec "$go_binary" test "$@"
