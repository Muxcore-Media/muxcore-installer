#!/usr/bin/env bash
# Bootstrap local admin user + session token (installer host stack).
# Uses bin/authctl and bin/gettoken; may try sibling source builds if missing.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
BIN="$ROOT/bin"

# shellcheck disable=SC1091
[[ -f "$ROOT/.env" ]] && source "$ROOT/.env" || true

USER="${MVP_ADMIN_USER:-admin}"
PASS="${MVP_ADMIN_PASSWORD:-}"
if [[ -z "$PASS" ]]; then
  echo "FAIL: MVP_ADMIN_PASSWORD is required (run muxcore-setup or set in .env)" >&2
  exit 1
fi
AUTH_ADDR="${AUTH_GRPC_ADDR:-127.0.0.1:9403}"
TOKEN_FILE="${MVP_TOKEN_FILE:-$ROOT/run/admin.token}"
[[ "$TOKEN_FILE" != /* ]] && TOKEN_FILE="$ROOT/${TOKEN_FILE#./}"

mkdir -p "$ROOT/run" "$BIN"

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

missing=()
ensure_helper authctl || missing+=(authctl)
ensure_helper gettoken || missing+=(gettoken)
if ((${#missing[@]} > 0)); then
  cat >&2 <<EOF
FAIL: missing helper CLI(s) under $BIN: ${missing[*]}

Place release/lab binaries into bin/, or:
  export MUXCORE_LAB_BIN=/path/to/_mvp/bin && ./install.sh
  # or build siblings when present:
  #   (cd ../auth-local && go build -o ../muxcore-installer/bin/authctl ./cmd/authctl)
  #   (cd ../_mvp && go build -o ../muxcore-installer/bin/gettoken ./cmd/gettoken)

This installer does not assume a full monorepo checkout.
EOF
  exit 1
fi

echo "==> ensuring user $USER exists"
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
"$BIN/gettoken" -addr "$AUTH_ADDR" -user "$USER" -password "$PASS" -out "$TOKEN_FILE"
echo "token written to $TOKEN_FILE"
echo "password (printed once for local demo): $PASS"
