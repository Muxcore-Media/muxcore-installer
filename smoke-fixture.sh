#!/usr/bin/env bash
# Offline fixture smoke for installer — no pirate APIs, no live torrents.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
# shellcheck disable=SC1091
[[ -f "$ROOT/.env" ]] && source "$ROOT/.env" || true

CORE_URL="${SMOKE_CORE_URL:-http://127.0.0.1:8080}"
API_URL="${SMOKE_API_URL:-http://127.0.0.1:18080}"
ADMIN_URL="${SMOKE_ADMIN_URL:-http://localhost:8082}"
ENGINE="${DOWNLOADER_ENGINE:-fixture}"

if [[ "${SMOKE_LIVE_ACQUISITION:-0}" == "1" ]]; then
  echo "NOTE: SMOKE_LIVE_ACQUISITION=1 is set but smoke-fixture.sh never calls pirate APIs."
  echo "      Use the _mvp lab live path only as operator opt-in; not a product gate."
fi

echo "==> smoke-fixture (engine=$ENGINE)"
code=$(curl -s -o /dev/null -w '%{http_code}' "$CORE_URL/health" || echo 000)
echo "core health: $code ($CORE_URL/health)"
if [[ "$code" != "200" && "$code" != "503" ]]; then
  echo "FAIL: core not healthy — run ./up.sh first" >&2
  exit 1
fi

api=$(curl -s -o /dev/null -w '%{http_code}' "$API_URL/api/v1/health" || echo 000)
echo "api health:  $api ($API_URL/api/v1/health)"

admin=$(curl -s -o /dev/null -w '%{http_code}' "$ADMIN_URL/health" || echo 000)
echo "admin:       $admin ($ADMIN_URL/health)"

TOKEN_FILE="${MVP_TOKEN_FILE:-$ROOT/run/admin.token}"
if [[ -f "$TOKEN_FILE" ]]; then
  tok=$(cat "$TOKEN_FILE")
  movies=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $tok" "$API_URL/api/movies" || echo 000)
  echo "api movies:  $movies (Bearer token)"
else
  echo "api movies:  skipped (no $TOKEN_FILE — run ./bootstrap-auth.sh)"
fi

cat <<EOF

Fixture notes
  - DOWNLOADER_ENGINE=fixture (default) — no BitTorrent / no pirate indexers
  - TMDB_FIXTURE=1 for offline metadata
  - Full acquire→library path: use _mvp/smoke.sh against this stack when bins present,
    or Dispatch fixture via admin /automation once modules are registered

VIEW-ME: $ROOT/run/VIEW-ME.txt
EOF

echo "==> smoke-fixture OK (core reachable)"
exit 0
