#!/usr/bin/env bash
# Regenerate assets/banner.ascii (figlet "big" font).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
nix-shell -p figlet --run "figlet -f big MuxCore" >"$ROOT/assets/banner.ascii"
# Trim trailing blank lines that figlet adds.
if command -v sed >/dev/null 2>&1; then
  sed -i -e :a -e '/^\n*$/{$d;N;ba' -e '}' "$ROOT/assets/banner.ascii" 2>/dev/null || true
fi
echo "wrote $ROOT/assets/banner.ascii"
