#!/usr/bin/env bash
# Non-interactive admin bootstrap for installer stack.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
# shellcheck disable=SC1091
[[ -f "$ROOT/.env" ]] && source "$ROOT/.env" || true

USER="${MVP_ADMIN_USER:-admin}"
PASS="${MVP_ADMIN_PASSWORD:-admin-dev-only}"
AUTH_ADDR="${AUTH_GRPC_ADDR:-127.0.0.1:9403}"
TOKEN_FILE="${MVP_TOKEN_FILE:-$ROOT/run/admin.token}"
BIN="$ROOT/bin"

mkdir -p "$ROOT/run"

ensure_helper() {
  local name="$1"
  if [[ -x "$BIN/$name" ]]; then
    return 0
  fi
  if [[ -n "${MUXCORE_LAB_BIN:-}" && -x "${MUXCORE_LAB_BIN}/$name" ]]; then
    install -m 0755 "${MUXCORE_LAB_BIN}/$name" "$BIN/$name"
    return 0
  fi
  if [[ -x "$ROOT/../_mvp/bin/$name" ]]; then
    install -m 0755 "$ROOT/../_mvp/bin/$name" "$BIN/$name"
    return 0
  fi
  if command -v go >/dev/null 2>&1; then
    case "$name" in
      authctl)
        if [[ -d "$ROOT/../auth-local/cmd/authctl" ]]; then
          (cd "$ROOT/../auth-local" && go build -o "$BIN/authctl" ./cmd/authctl)
          return 0
        fi
        ;;
      gettoken)
        if [[ -d "$ROOT/../_mvp/cmd/gettoken" ]]; then
          (cd "$ROOT/../_mvp" && go build -o "$BIN/gettoken" ./cmd/gettoken)
          return 0
        fi
        ;;
    esac
  fi
  echo "FAIL: bin/$name missing. Re-run ./install.sh with lab bins or place $name in bin/" >&2
  exit 1
}

ensure_helper authctl
ensure_helper gettoken

echo "==> ensuring user $USER exists"
if ! "$BIN/authctl" -addr "$AUTH_ADDR" adduser "$USER" "$PASS" 2>/tmp/muxcore-installer-adduser.err; then
  if grep -qiE 'already|exists|unique|duplicate' /tmp/muxcore-installer-adduser.err; then
    echo "user already exists"
  else
    cat /tmp/muxcore-installer-adduser.err >&2
    exit 1
  fi
fi

echo "==> ensuring admin role"
"$BIN/authctl" -addr "$AUTH_ADDR" addrole "$USER" admin || true

echo "==> fetching session token"
"$BIN/gettoken" -addr "$AUTH_ADDR" -user "$USER" -password "$PASS" -out "$TOKEN_FILE"
echo "token written to $TOKEN_FILE"
echo "login: $USER / $PASS (change for anything beyond laptop demo)"
