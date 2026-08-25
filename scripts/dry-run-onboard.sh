#!/usr/bin/env bash
# Non-interactive dry run of the wizard (stops before download/start).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export MUXCORE_I_AGREE=1
export MUXCORE_NONINTERACTIVE=1
export MUXCORE_DRY_RUN=1
exec bash "$ROOT/onboard.sh"
