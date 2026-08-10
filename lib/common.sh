# Shared helpers for muxcore-installer scripts.
# shellcheck shell=bash

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
  local pass="${MVP_ADMIN_PASSWORD:-admin-dev-only}"
  cat >"$out" <<EOF
MuxCore installer — VIEW-ME

  Admin UI:     http://localhost:8082
                login: ${user} / ${pass}

  Consumer UI:  http://127.0.0.1:5173
                (optional; needs media-ui dist + mediauiprox)

  Core health:  http://127.0.0.1:8080/health
  REST API:     http://127.0.0.1:18080/api/v1/health
  Jellyfin:     http://127.0.0.1:8475/healthz
  Monitor:      http://127.0.0.1:9203/status

Useful admin pages
  /dashboard/monitor
  /modules
  /automation
  /jellyfin
  /events?filter=health

Fixture demo
  DOWNLOADER_ENGINE=fixture (default) — no pirate indexers / no live torrents
  TMDB_FIXTURE=1 for offline metadata

Start:  ./up.sh
Stop:   ./up.sh stop
Auth:   ./bootstrap-auth.sh
Smoke:  ./smoke-fixture.sh
EOF
  echo "wrote $out"
}
