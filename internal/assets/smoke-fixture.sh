#!/usr/bin/env bash
# Offline health smoke for the installer stack.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
if [[ -f "$ROOT/.env" ]]; then
  # shellcheck disable=SC1091
  source "$ROOT/.env"
fi

# shellcheck disable=SC1091
source "$ROOT/lib/common.sh"
resolve_security_profile 0 || exit 1

BIN="$ROOT/bin"
# household: core's HTTP port is HTTPS, verified against the CA it exports.
CURL_CORE=()
if [[ "$SEC_PROFILE" == household ]]; then
  CORE_URL="${SMOKE_CORE_URL:-https://127.0.0.1:8080}"
  CURL_CORE=(--cacert "$ROOT/mesh/public/ca.crt")
else
  CORE_URL="${SMOKE_CORE_URL:-http://127.0.0.1:8080}"
fi
API_URL="${SMOKE_API_URL:-http://127.0.0.1:18080}"
ADMIN_URL="${SMOKE_ADMIN_URL:-http://localhost:8082}"
TIMEOUT="${SMOKE_TIMEOUT_SEC:-120}"
TOKEN_FILE="${MVP_TOKEN_FILE:-$ROOT/run/admin.token}"
[[ "$TOKEN_FILE" != /* ]] && TOKEN_FILE="$ROOT/${TOKEN_FILE#./}"

# Hard refuse unsupported live acquisition flags if someone sets them.
if [[ -n "${PIRATEBAY_API_BASE:-}" || "${SMOKE_LIVE_ACQUISITION:-}" == "1" ]]; then
  echo "WARN: acquisition-related env vars are set but ignored by this health check." >&2
fi

echo "==> smoke-fixture: first-run health check"

echo "==> checking release/lab binaries in $BIN"
REQUIRED=(muxcored api-rest auth-local)
MISSING=()
for b in "${REQUIRED[@]}"; do
  if [[ ! -x "$BIN/$b" ]]; then
    MISSING+=("$b")
  fi
done
if ((${#MISSING[@]})); then
  cat >&2 <<EOF
FAIL: missing binaries in $BIN: ${MISSING[*]}

The full stack cannot start without module binaries.
Place GitHub Release binaries into bin/ (preferred), or:

  export MUXCORE_LAB_BIN=/path/to/MuxCore/_mvp/bin
  ./install.sh

Then: ./up.sh && ./bootstrap-auth.sh && ./smoke-fixture.sh
EOF
  exit 1
fi
echo "OK essential binaries present (${REQUIRED[*]})"

echo "==> checking core health at ${CORE_URL}/health"
code=$(curl -s ${CURL_CORE[@]+"${CURL_CORE[@]}"} -o /tmp/muxcore-installer-core-health.json -w '%{http_code}' "${CORE_URL}/health" || echo 000)
if [[ "$code" != "200" && "$code" != "503" && "$SEC_PROFILE" == household && -z "${SMOKE_CORE_URL:-}" ]]; then
  # A core started with `./up.sh --dev` answers plain HTTP; do not restart it.
  plain=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/health || echo 000)
  if [[ "$plain" == "200" || "$plain" == "503" ]]; then
    echo "FAIL: core answers plain HTTP (dev profile, e.g. ./up.sh --dev) but .env configures household; restart with ./up.sh" >&2
    exit 1
  fi
fi
if [[ "$code" != "200" && "$code" != "503" ]]; then
  echo "core not up (HTTP $code). Attempting ./up.sh ..."
  if ! "$ROOT/up.sh" up; then
    echo "FAIL: could not start stack. Ensure binaries are present (see install summary)." >&2
    exit 1
  fi
fi

echo "==> waiting for core ${CORE_URL}/health (timeout ${TIMEOUT}s)"
deadline=$((SECONDS + TIMEOUT))
until code=$(curl -s ${CURL_CORE[@]+"${CURL_CORE[@]}"} -o /tmp/muxcore-installer-core-health.json -w '%{http_code}' "${CORE_URL}/health" || echo 000); \
  [[ "$code" == "200" || "$code" == "503" ]]; do
  if (( SECONDS >= deadline )); then
    echo "FAIL: core health not ready (last HTTP $code)" >&2
    cat /tmp/muxcore-installer-core-health.json 2>/dev/null || true
    echo "Check $ROOT/run/core.log — if modules failed, place release binaries and re-run install/up." >&2
    exit 1
  fi
  sleep 2
done
echo "OK core health (HTTP $code)"

# Security profile (ADR-0016): core reports it on /health.
health_profile="$(sed -n 's/.*"profile":[[:space:]]*"\([a-z]*\)".*/\1/p' /tmp/muxcore-installer-core-health.json 2>/dev/null || true)"
if [[ -n "$health_profile" && "$health_profile" != "$SEC_PROFILE" ]]; then
  echo "FAIL: core runs the $health_profile profile, but this install is configured for $SEC_PROFILE" >&2
  exit 1
fi
if [[ "$SEC_PROFILE" == household ]]; then
  echo "OK core profile household (TLS, verified against mesh/public/ca.crt)"
else
  echo "WARN: core runs the DEV security profile (plaintext mesh) — development only" >&2
fi

echo "==> waiting for api-rest ${API_URL}/api/v1/health"
until curl -sf "${API_URL}/api/v1/health" >/dev/null 2>&1; do
  if (( SECONDS >= deadline )); then
    echo "FAIL: api-rest health not ready" >&2
    echo "Is bin/api-rest present and started? See run/api-rest.log" >&2
    exit 1
  fi
  sleep 2
done
echo "OK api-rest health"

if [[ -x "$BIN/admin-ui" ]]; then
  echo "==> admin-ui ${ADMIN_URL}/health"
  adm_deadline=$((SECONDS + 30))
  until code=$(curl -s -o /dev/null -w '%{http_code}' "${ADMIN_URL}/health" || echo 000); \
    [[ "$code" == "200" ]]; do
    if (( SECONDS >= adm_deadline )); then
      echo "WARN: admin-ui health not HTTP 200 (last $code) — continue" >&2
      break
    fi
    sleep 1
  done
  [[ "${code:-}" == "200" ]] && echo "OK admin-ui health"
fi

if [[ ! -f "$TOKEN_FILE" ]]; then
  if [[ -x "$BIN/authctl" && -x "$BIN/gettoken" ]]; then
    echo "==> no token yet; running bootstrap-auth.sh"
    "$ROOT/bootstrap-auth.sh" || echo "WARN: bootstrap-auth failed (auth may still be warming)" >&2
  else
    echo "WARN: no token and missing authctl/gettoken — skip authenticated checks" >&2
  fi
fi

if [[ -f "$TOKEN_FILE" ]]; then
  TOKEN="$(tr -d '\n' <"$TOKEN_FILE")"
  if [[ -n "$TOKEN" ]]; then
    echo "==> GET ${API_URL}/api/v1/modules (bearer)"
    mods="$(curl -sf -H "Authorization: Bearer ${TOKEN}" "${API_URL}/api/v1/modules" || true)"
    if [[ -n "$mods" ]]; then
      echo "$mods" | head -c 400
      echo
      echo "OK authenticated modules list"
    else
      echo "WARN: modules list empty/failed — auth or discovery may still be warming up" >&2
    fi
  fi
fi

if [[ "$SEC_PROFILE" == household ]]; then
  # Every module this install started must have enrolled (ADR-0017).
  echo "==> mesh identities (mesh/id)"
  missing_ids=()
  enrolled=0
  for pidf in "$ROOT"/run/*.pid; do
    [[ -f "$pidf" ]] || continue
    name="$(basename "$pidf" .pid)"
    [[ "$name" == core ]] && continue
    if ! kill -0 "$(cat "$pidf")" 2>/dev/null; then
      [[ -s "$ROOT/mesh/id/$name/module.crt" ]] ||
        echo "WARN: $name is not running and has no mesh identity — see run/$name.log" >&2
      continue
    fi
    id_deadline=$((SECONDS + 30))
    until [[ -s "$ROOT/mesh/id/$name/module.crt" ]] || ((SECONDS >= id_deadline)); do
      sleep 1
    done
    if [[ -s "$ROOT/mesh/id/$name/module.crt" ]]; then
      enrolled=$((enrolled + 1))
    else
      missing_ids+=("$name")
    fi
  done
  if ((${#missing_ids[@]})); then
    echo "FAIL: no mesh identity for: ${missing_ids[*]} (see run/<module>.log)" >&2
    exit 1
  fi
  echo "OK $enrolled running modules enrolled with core's CA"
fi

cat <<EOF

======== health check notes ========
Security profile: ${SEC_PROFILE}
TMDB_FIXTURE=${TMDB_FIXTURE:-1}
Movie library: ${MVP_LIBRARY_ROOT:-$ROOT/data/library}
TV library:    ${MVP_TV_LIBRARY_ROOT:-$ROOT/data/library/tv}
Import folder: ${MVP_IMPORT_DIR:-${MVP_DOWNLOADS_DIR:-$ROOT/data/import}}

URLs: $ROOT/run/VIEW-ME.txt
Admin: ${ADMIN_URL}

PASS: installer health smoke
EOF
