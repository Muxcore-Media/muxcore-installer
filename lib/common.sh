# Shared helpers for muxcore-installer scripts.
# shellcheck shell=bash

_ONBOARD_READ_FD=0
_ONBOARD_TTY_OPENED=0

# curl | bash leaves stdin at EOF; read answers from the controlling TTY instead.
onboard_open_tty() {
  if [[ "$_ONBOARD_TTY_OPENED" == 1 ]]; then
    return 0
  fi
  if [[ -r /dev/tty ]]; then
    exec 3</dev/tty
    _ONBOARD_READ_FD=3
    _ONBOARD_TTY_OPENED=1
    return 0
  fi
  if [[ ! -t 0 ]]; then
    echo "error: interactive setup requires a terminal (no /dev/tty)." >&2
    echo "Save the script and run it directly:" >&2
    echo "  curl -fsSL https://getmuxcore.zem.systems -o get-onboard.sh && bash get-onboard.sh" >&2
    return 1
  fi
  _ONBOARD_READ_FD=0
  _ONBOARD_TTY_OPENED=1
  return 0
}

onboard_require_tty() {
  onboard_open_tty || exit 1
}

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
  local pass="${MVP_ADMIN_PASSWORD:-admin-dev-only}"
  cat >"$out" <<EOF
MuxCore — you're ready

  Admin UI:     http://localhost:8082
                login: ${user} / ${pass}

  Consumer UI:  http://127.0.0.1:5173
                (optional; enable with MVP_ENABLE_MEDIA_UI=1 + media-ui dist)

  Core health:  http://127.0.0.1:8080/health
  REST API:     http://127.0.0.1:18080/api/v1/health
  Jellyfin:     http://127.0.0.1:8475/healthz
  Monitor:      http://127.0.0.1:9203/status

  Movie library: ${MVP_LIBRARY_ROOT:-$root/data/library}
  TV library:    ${MVP_TV_LIBRARY_ROOT:-$root/data/library/tv}
  Incoming:      ${MVP_DOWNLOADS_DIR:-$root/data/downloads}

Useful admin pages
  /dashboard/monitor
  /modules
  /automation
  /jellyfin
  /events?filter=health

Metadata
  TMDB_FIXTURE=${TMDB_FIXTURE:-1} (offline demo titles when no API key)

Start:  ./up.sh
Stop:   ./up.sh stop
Auth:   ./bootstrap-auth.sh
Smoke:  ./smoke-fixture.sh
EOF
  echo "wrote $out"
}

onboard_step() {
  printf '\n── Step %s/%s — %s ──\n' "$1" "$2" "$3"
}

onboard_prompt() {
  local var="$1" q="$2" def="${3:-}" ans
  onboard_require_tty
  if [[ -n "$def" ]]; then
    read -r -p "$q [$def]: " ans -u "$_ONBOARD_READ_FD" || true
    ans="${ans:-$def}"
  else
    read -r -p "$q: " ans -u "$_ONBOARD_READ_FD" || true
  fi
  printf -v "$var" '%s' "$ans"
}

onboard_prompt_secret() {
  local var="$1" q="$2" def="${3:-}" ans
  onboard_require_tty
  if [[ -n "$def" ]]; then
    read -r -s -p "$q [press Enter to keep current]: " ans -u "$_ONBOARD_READ_FD" || true
    echo >&"$_ONBOARD_READ_FD"
    ans="${ans:-$def}"
  else
    read -r -s -p "$q: " ans -u "$_ONBOARD_READ_FD" || true
    echo >&"$_ONBOARD_READ_FD"
    ans="${ans:-$def}"
  fi
  printf -v "$var" '%s' "$ans"
}

onboard_yesno() {
  local q="$1" def="${2:-y}" ans
  onboard_require_tty
  read -r -p "$q [$def]: " ans -u "$_ONBOARD_READ_FD" || true
  ans="${ans:-$def}"
  [[ "$ans" =~ ^[Yy] ]]
}

env_set() {
  local envf="$1" key="$2" val="$3"
  local tmp
  tmp="$(mktemp)"
  touch "$envf"
  if grep -q "^${key}=" "$envf" 2>/dev/null; then
    # shellcheck disable=SC2016
    awk -v k="$key" -v v="$val" 'BEGIN{FS=OFS="="} $1==k{$0=k"="v} {print}' "$envf" >"$tmp"
    mv "$tmp" "$envf"
  else
    printf '%s=%s\n' "$key" "$val" >>"$envf"
    rm -f "$tmp"
  fi
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
