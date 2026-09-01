# Shared helpers for muxcore-installer scripts.
# shellcheck shell=bash

ensure_writable_dir() {
  local d="$1"
  if [[ -e "$d" && ! -d "$d" ]]; then
    echo "error: $d exists and is not a directory" >&2
    return 1
  fi
  if ! mkdir -p "$d" 2>/dev/null; then
    echo "error: cannot create $d (permission denied)" >&2
    return 1
  fi
  if [[ ! -w "$d" ]]; then
    echo "error: directory not writable: $d" >&2
    return 1
  fi
  return 0
}

installer_root() {
  cd "$(dirname "${BASH_SOURCE[1]}")/.." && pwd
}

detect_os_arch() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *)
      echo "unsupported arch: $arch" >&2
      return 1
      ;;
  esac
  case "$os" in
    linux|darwin) ;;
    *)
      echo "unsupported os: $os" >&2
      return 1
      ;;
  esac
  printf '%s %s\n' "$os" "$arch"
}

resolve_lab_bin() {
  # Prefer explicit override, then sibling MuxCore/_mvp/bin (laptop lab).
  if [[ -n "${MUXCORE_LAB_BIN:-}" ]]; then
    printf '%s\n' "$MUXCORE_LAB_BIN"
    return 0
  fi
  local root="$1"
  local candidate
  for candidate in \
    "$root/../_mvp/bin" \
    "$root/../../_mvp/bin" \
    "$HOME/Projects/MuxCore/_mvp/bin"; do
    if [[ -d "$candidate" ]]; then
      cd "$candidate" && pwd
      return 0
    fi
  done
  return 1
}

require_cmd() {
  local c
  for c in "$@"; do
    command -v "$c" >/dev/null 2>&1 || {
      echo "missing required command: $c" >&2
      return 1
    }
  done
}

write_view_me() {
  local root="$1"
  local out="${2:-$root/run/VIEW-ME.txt}"
  mkdir -p "$(dirname "$out")"
  local user="${MVP_ADMIN_USER:-admin}"
  local pass="${MVP_ADMIN_PASSWORD:-}"
  local player=""
  if [[ "${MVP_ENABLE_MEDIA_UI:-0}" != "0" ]]; then
    player="  Player:       http://127.0.0.1:5173"
  fi
  cat >"$out" <<EOF
MuxCore — you're ready

  Admin UI:     http://localhost:8082
                login: ${user} / ${pass}
${player}

  Core health:  http://127.0.0.1:8080/health
  REST API:     http://127.0.0.1:18080/api/v1/health
  Monitor:      http://127.0.0.1:9203/status

  Movie library: ${MVP_LIBRARY_ROOT:-$root/data/library}
  TV library:    ${MVP_TV_LIBRARY_ROOT:-$root/data/library/tv}
  Import folder: ${MVP_IMPORT_DIR:-${MVP_DOWNLOADS_DIR:-$root/data/import}}

Useful admin pages
  /dashboard/monitor
  /modules
  /events?filter=health

Metadata
  TMDB_FIXTURE=${TMDB_FIXTURE:-1} (offline sample titles when no API key)

Start:  ./up.sh
Stop:   ./up.sh stop
Auth:   ./bootstrap-auth.sh
Smoke:  ./smoke-fixture.sh

Admin API token: run/admin.token (muxcorectl / REST when installed)
Re-read:         cat run/VIEW-ME.txt
EOF
  echo "wrote $out"
}

env_set() {
  local envf="$1" key="$2" val="$3"
  local tmp
  tmp="$(mktemp)"
  touch "$envf"
  grep -v "^${key}=" "$envf" >"$tmp" || true
  printf '%s=%s\n' "$key" "$val" >>"$tmp"
  mv "$tmp" "$envf"
}

resolve_install_dir() {
  local raw="$1"
  local resolved
  if resolved="$(cd / && realpath -m "$raw" 2>/dev/null)"; then
    printf '%s\n' "$resolved"
  elif resolved="$(python3 -c 'import os,sys; print(os.path.abspath(os.path.expanduser(sys.argv[1])))' "$raw" 2>/dev/null)"; then
    printf '%s\n' "$resolved"
  else
    case "$raw" in
      ~/*) printf '%s\n' "${HOME}/${raw#~/}" ;;
      ~) printf '%s\n' "$HOME" ;;
      *) printf '%s\n' "$raw" ;;
    esac
  fi
}
