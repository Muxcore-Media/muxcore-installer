#!/usr/bin/env bash
# Bootstrap the local admin login + session token (installer host stack).
#
# auth-local creates the admin (with the admin role) on an empty database from
# AUTH_BOOTSTRAP_USER/AUTH_BOOTSTRAP_PASSWORD, which up.sh sets from
# MVP_ADMIN_USER/MVP_ADMIN_PASSWORD. This script logs in over auth-local's HTTP
# device-login endpoint (POST /login/device, JSON) and writes the session token
# to run/admin.token. That works in both security profiles: the mesh gRPC port
# is TLS-only in household, and the gettoken helper speaks plaintext only.
#
# Fallback when the login fails (e.g. an auth-local without the bootstrap):
# create the user with bin/authctl (TLS against core's CA in household), then
# log in again; in dev, bin/gettoken is the last resort.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
BIN="$ROOT/bin"

if [[ -f "$ROOT/.env" ]]; then
  # shellcheck disable=SC1091
  source "$ROOT/.env"
fi
# shellcheck disable=SC1091
source "$ROOT/lib/common.sh"

USER="${MVP_ADMIN_USER:-admin}"
PASS="${MVP_ADMIN_PASSWORD:-}"
if [[ -z "$PASS" ]]; then
  echo "FAIL: MVP_ADMIN_PASSWORD is required (run muxcore-setup or set in .env)" >&2
  exit 1
fi
AUTH_ADDR="${AUTH_GRPC_ADDR:-127.0.0.1:9403}"
AUTH_HTTP="${AUTH_HTTP_URL:-http://127.0.0.1:9401}"
TOKEN_FILE="${MVP_TOKEN_FILE:-$ROOT/run/admin.token}"
[[ "$TOKEN_FILE" != /* ]] && TOKEN_FILE="$ROOT/${TOKEN_FILE#./}"
LOGIN_WAIT_SEC="${BOOTSTRAP_LOGIN_WAIT_SEC:-30}"

resolve_security_profile 0 || exit 1
if [[ "$SEC_PROFILE" == household ]]; then
  # authctl (fallback path) verifies auth-local's core-issued certificate.
  export MUXCORE_TLS_CA="$ROOT/mesh/public/ca.crt"
  unset MUXCORE_INSECURE_DISABLE_TLS MUXCORE_GRPC_INSECURE MUXCORE_DEV_TLS_SKIP
else
  export MUXCORE_INSECURE_DISABLE_TLS=true
fi

mkdir -p "$ROOT/run" "$BIN"

json_str() {
  local s="$1"
  s="${s//\\/\\\\}"
  s="${s//\"/\\\"}"
  printf '"%s"' "$s"
}

# login_device: POST credentials to auth-local's device login; on success the
# session token is written to TOKEN_FILE (0600). The password goes through
# stdin, never a command line.
login_device() {
  local resp token
  resp="$(printf '{"username":%s,"password":%s}' "$(json_str "$USER")" "$(json_str "$PASS")" |
    curl -sS --max-time 10 -H 'Content-Type: application/json' --data-binary @- \
      "$AUTH_HTTP/login/device" 2>/dev/null)" || return 1
  if [[ "$resp" == *'"requires_2fa"'*true* ]]; then
    echo "FAIL: $USER has TOTP enabled; log in through the admin UI instead" >&2
    return 2
  fi
  token="$(printf '%s' "$resp" | sed -n 's/.*"token":[[:space:]]*"\([^"]*\)".*/\1/p')"
  [[ -n "$token" ]] || return 1
  (umask 077 && printf '%s\n' "$token" >"$TOKEN_FILE")
  chmod 600 "$TOKEN_FILE"
  return 0
}

login_with_retry() {
  local deadline=$((SECONDS + $1)) rc
  while :; do
    rc=0
    login_device || rc=$?
    [[ "$rc" -eq 0 ]] && return 0
    [[ "$rc" -eq 2 ]] && exit 1
    ((SECONDS >= deadline)) && return 1
    sleep 1
  done
}

ensure_helper() {
  local name="$1"
  if [[ -x "$BIN/$name" ]]; then
    return 0
  fi

  # Lab copy
  if [[ -n "${MUXCORE_LAB_BIN:-}" && -x "${MUXCORE_LAB_BIN}/$name" ]]; then
    install -m 0755 "${MUXCORE_LAB_BIN}/$name" "$BIN/$name"
    echo "==> copied $name from MUXCORE_LAB_BIN"
    return 0
  fi
  local lab
  for lab in "$ROOT/../_mvp/bin" "$HOME/Projects/MuxCore/_mvp/bin"; do
    if [[ -x "$lab/$name" ]]; then
      install -m 0755 "$lab/$name" "$BIN/$name"
      echo "==> copied $name from $lab"
      return 0
    fi
  done

  # Optional sibling Go build (do not assume monorepo always exists).
  if ! command -v go >/dev/null 2>&1; then
    return 1
  fi
  case "$name" in
    authctl)
      if [[ -d "$ROOT/../auth-local/cmd/authctl" ]]; then
        echo "==> building authctl from sibling auth-local"
        (cd "$ROOT/../auth-local" && go build -o "$BIN/authctl" ./cmd/authctl)
        return 0
      fi
      ;;
    gettoken)
      if [[ -d "$ROOT/../_mvp/cmd/gettoken" ]]; then
        echo "==> building gettoken from sibling _mvp"
        (cd "$ROOT/../_mvp" && go build -o "$BIN/gettoken" ./cmd/gettoken)
        return 0
      fi
      ;;
  esac
  return 1
}

done_ok() {
  echo "token written to $TOKEN_FILE"
  echo "password (printed once for local demo): $PASS"
  exit 0
}

echo "==> logging in as $USER ($AUTH_HTTP/login/device; security profile $SEC_PROFILE)"
if login_with_retry "$LOGIN_WAIT_SEC"; then
  done_ok
fi

echo "==> login failed; ensuring user $USER exists with authctl"
if ! ensure_helper authctl; then
  cat >&2 <<EOF
FAIL: could not log in as $USER at $AUTH_HTTP, and bin/authctl is missing.

Is auth-local running? See $ROOT/run/auth-local.log.
auth-local creates $USER on an empty database from MVP_ADMIN_USER/MVP_ADMIN_PASSWORD
in .env; if users already exist, log in with an existing admin instead.
EOF
  exit 1
fi
err="$(mktemp)"
trap 'rm -f "$err"' EXIT
if ! "$BIN/authctl" -addr "$AUTH_ADDR" adduser "$USER" "$PASS" 2>"$err"; then
  if grep -qiE 'already|exists|unique|duplicate' "$err"; then
    echo "user already exists"
  else
    cat "$err" >&2
    exit 1
  fi
fi

echo "==> ensuring admin role"
"$BIN/authctl" -addr "$AUTH_ADDR" addrole "$USER" admin || true

echo "==> fetching session token"
if login_with_retry 5; then
  done_ok
fi
if [[ "$SEC_PROFILE" == dev ]] && ensure_helper gettoken; then
  "$BIN/gettoken" -addr "$AUTH_ADDR" -user "$USER" -password "$PASS" -out "$TOKEN_FILE"
  done_ok
fi
echo "FAIL: could not obtain a session token for $USER (see $ROOT/run/auth-local.log)" >&2
exit 1
