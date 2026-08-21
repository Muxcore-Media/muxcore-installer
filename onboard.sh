#!/usr/bin/env bash
# Interactive first-run onboarding for MuxCore (non-developer path).
# Installs default platform + media modules, configures storage, creates admin, starts stack.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
ENVF="$ROOT/.env"
EXAMPLE="$ROOT/.env.example"
TOTAL_STEPS=7

# shellcheck disable=SC1091
source "$ROOT/lib/common.sh"

die() { echo "error: $*" >&2; exit 1; }
info() { echo "==> $*"; }
ok() { echo "    $*"; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1 (install it, then re-run onboarding)"
}

load_env() {
  [[ -f "$ENVF" ]] || return 0
  set -a
  # shellcheck disable=SC1090
  source "$ENVF" >/dev/null 2>&1 || true
  set +a
}

ensure_env_file() {
  if [[ ! -f "$ENVF" ]]; then
    [[ -f "$EXAMPLE" ]] || die "missing .env.example in $ROOT"
    cp "$EXAMPLE" "$ENVF"
    ok "created .env from defaults"
  else
    ok "updating existing .env"
  fi
}

step_prerequisites() {
  onboard_step 1 "$TOTAL_STEPS" "Check your computer"
  echo "MuxCore runs on Linux or macOS (Intel or Apple Silicon)."
  echo "You need curl and tar. Go and Docker are optional."
  echo
  need_cmd bash
  need_cmd curl
  need_cmd tar
  read -r _os _arch < <(detect_os_arch)
  ok "platform ${_os}/${_arch}"
  if command -v docker >/dev/null 2>&1; then
    ok "Docker found (optional — not required for this path)"
  fi
  if command -v gh >/dev/null 2>&1; then
    ok "GitHub CLI found (helps fetch private release assets)"
  fi
}

step_install_dir() {
  onboard_step 2 "$TOTAL_STEPS" "Choose where MuxCore lives"
  local current="$ROOT"
  local target
  onboard_prompt target "Install folder" "${MUXCORE_INSTALL_DIR:-$current}"
  target="$(resolve_install_dir "$target")"
  mkdir -p "$target"
  if [[ "$(cd "$target" && pwd)" != "$ROOT" ]]; then
    info "copying installer into $target"
    if command -v rsync >/dev/null 2>&1; then
      rsync -a --exclude data --exclude run --exclude bin --exclude cache \
        "$ROOT/" "$target/"
    else
      mkdir -p "$target"
      tar -C "$ROOT" -cf - . | tar -C "$target" -xf -
    fi
    ok "continuing in $target"
    exec "$target/onboard.sh" "$@"
  fi
  ok "using $ROOT"
}

step_fetch_binaries() {
  onboard_step 3 "$TOTAL_STEPS" "Get MuxCore components"
  echo "This downloads the core and default modules (auth, libraries, scanner, admin UI, …)."
  echo "No special acquisition modules are included."
  echo
  if [[ -x "$ROOT/bin/muxcored" && -x "$ROOT/bin/api-rest" && -x "$ROOT/bin/auth-local" ]]; then
    if onboard_yesno "Binaries already present in bin/. Re-download?" "n"; then
      :
    else
      ok "keeping existing binaries"
      return 0
    fi
  fi
  if [[ -d "$ROOT/../_mvp/bin" && -x "$ROOT/../_mvp/bin/muxcored" ]]; then
    if onboard_yesno "Use built binaries from sibling _mvp/bin (developer lab)?" "n"; then
      export MUXCORE_LAB_BIN="$(cd "$ROOT/../_mvp/bin" && pwd)"
      ok "lab binaries: $MUXCORE_LAB_BIN"
    fi
  fi
  if [[ -z "${MUXCORE_LAB_BIN:-}" ]] && onboard_yesno "Download release binaries now?" "y"; then
    info "running install.sh (may take a few minutes on first run)"
    "$ROOT/install.sh"
  elif [[ -z "${MUXCORE_LAB_BIN:-}" ]]; then
    ok "skipped download — place binaries in $ROOT/bin/ then run ./install.sh"
  else
    "$ROOT/install.sh"
  fi
}

step_metadata() {
  onboard_step 4 "$TOTAL_STEPS" "Movie & TV metadata"
  echo "MuxCore uses TMDB for posters, descriptions, and search."
  echo "  1) I have a free TMDB API key (recommended)"
  echo "  2) Offline demo mode (Fight Club + Breaking Bad fixtures only)"
  local mode key
  onboard_prompt mode "Choose metadata mode" "2"
  case "$mode" in
    1)
      onboard_prompt_secret key "TMDB v3 API key" "${TMDB_API_KEY:-}"
      [[ -n "$key" ]] || die "TMDB API key required (get one at themoviedb.org, or choose mode 2)"
      env_set "$ENVF" TMDB_API_KEY "$key"
      env_set "$ENVF" TMDB_FIXTURE ""
      ok "live TMDB metadata enabled"
      ;;
    *)
      env_set "$ENVF" TMDB_FIXTURE 1
      env_set "$ENVF" TMDB_API_KEY ""
      ok "offline metadata fixtures enabled"
      ;;
  esac
}

step_admin() {
  onboard_step 5 "$TOTAL_STEPS" "Your admin account"
  echo "This is the username and password for the Admin UI (http://localhost:8082)."
  local user pass
  onboard_prompt user "Admin username" "${MVP_ADMIN_USER:-admin}"
  onboard_prompt_secret pass "Admin password" "${MVP_ADMIN_PASSWORD:-}"
  if [[ -z "$pass" ]]; then
    pass="$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 16 || true)"
    [[ -n "$pass" ]] || pass="muxcore-$(date +%s | tail -c 6)"
    ok "generated password: $pass"
  fi
  env_set "$ENVF" MVP_ADMIN_USER "$user"
  env_set "$ENVF" MVP_ADMIN_PASSWORD "$pass"
  export MVP_ADMIN_USER="$user"
  export MVP_ADMIN_PASSWORD="$pass"
}

step_storage() {
  onboard_step 6 "$TOTAL_STEPS" "Library folders"
  echo "Pick where movies, TV shows, and incoming imports are stored."
  echo "Folders are created automatically if they do not exist."
  local movies tv incoming
  onboard_prompt movies "Movie library folder" "${MVP_LIBRARY_ROOT:-$ROOT/data/library}"
  onboard_prompt tv "TV library folder" "${MVP_TV_LIBRARY_ROOT:-$ROOT/data/library/tv}"
  onboard_prompt incoming "Incoming imports folder (scanner watches here)" "${MVP_DOWNLOADS_DIR:-$ROOT/data/downloads}"
  movies="$(resolve_install_dir "$movies")"
  tv="$(resolve_install_dir "$tv")"
  incoming="$(resolve_install_dir "$incoming")"
  mkdir -p "$movies" "$tv" "$incoming"
  env_set "$ENVF" MVP_LIBRARY_ROOT "$movies"
  env_set "$ENVF" MVP_TV_LIBRARY_ROOT "$tv"
  env_set "$ENVF" MVP_DOWNLOADS_DIR "$incoming"
  env_set "$ENVF" MUXCORE_STORAGE_DIR "${MUXCORE_STORAGE_DIR:-$ROOT/data/storage}"
  ok "movies → $movies"
  ok "TV     → $tv"
  ok "incoming → $incoming"
}

step_finish() {
  onboard_step 7 "$TOTAL_STEPS" "Optional extras & launch"
  load_env

  if onboard_yesno "Connect an existing Jellyfin server now?" "n"; then
    local jf_url jf_key
    onboard_prompt jf_url "Jellyfin URL" "${JELLYFIN_BASE_URL:-http://127.0.0.1:8096}"
    onboard_prompt_secret jf_key "Jellyfin API key" "${JELLYFIN_API_KEY:-}"
    env_set "$ENVF" JELLYFIN_BASE_URL "$jf_url"
    env_set "$ENVF" JELLYFIN_API_KEY "$jf_key"
    ok "Jellyfin bridge configured"
  else
    ok "Jellyfin left unconfigured (soft bridge still starts — configure later in Admin UI)"
  fi

  env_set "$ENVF" MUXCORE_INSECURE_DISABLE_TLS true
  env_set "$ENVF" MVP_ENABLE_MEDIA_UI 0

  if onboard_yesno "Start MuxCore now?" "y"; then
    info "starting stack"
    "$ROOT/up.sh"
    info "waiting for core health"
    local deadline=$((SECONDS + 120)) code
    until code=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/health || echo 000); [[ "$code" == "200" ]]; do
      (( SECONDS < deadline )) || die "core /health did not become ready — check $ROOT/run/core.log"
      sleep 1
    done
    ok "core is healthy"
    info "creating your admin login"
    "$ROOT/bootstrap-auth.sh"
  else
    ok "skipped start — run ./up.sh && ./bootstrap-auth.sh when ready"
  fi

  load_env
  write_view_me "$ROOT"
  echo
  onboard_step 7 "$TOTAL_STEPS" "Done"
  cat "$ROOT/run/VIEW-ME.txt"
  echo
  if onboard_yesno "Run a quick health check?" "y"; then
    "$ROOT/smoke-fixture.sh" || true
  fi
  echo
  echo "Bookmark http://localhost:8082 and open Admin UI → Modules to confirm everything is green."
}

main() {
  cd "$ROOT"
  echo
  echo "╔══════════════════════════════════════════╗"
  echo "║  MuxCore setup — guided first launch     ║"
  echo "╚══════════════════════════════════════════╝"
  echo
  echo "This walkthrough installs the default platform and media modules,"
  echo "sets library folders, creates your admin account, and starts MuxCore."
  echo

  ensure_env_file
  load_env
  step_prerequisites
  step_install_dir "$@"
  step_fetch_binaries
  ensure_env_file
  step_metadata
  step_admin
  step_storage
  step_finish
}

main "$@"
