#!/usr/bin/env bash
# Regenerate assets/banner.ascii (figlet "big" font).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
nix-shell -p figlet --run "figlet -f big MuxCore" >"$ROOT/assets/banner.ascii"
echo "wrote $ROOT/assets/banner.ascii"
