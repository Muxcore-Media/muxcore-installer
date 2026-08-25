#!/usr/bin/env bash
# Interactive first-run onboarding for MuxCore (non-developer path).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
ENVF="$ROOT/.env"
EXAMPLE="$ROOT/.env.example"
TOTAL_STEPS=7

# shellcheck disable=SC1091
source "$ROOT/lib/common.sh"
# shellcheck disable=SC1091
source "$ROOT/lib/github.sh"
# shellcheck disable=SC1091
source "$ROOT/lib/ui.sh"
# shellcheck disable=SC1091
source "$ROOT/lib/prereqs.sh"

UI_ROOT="$ROOT"
PREREQS_ROOT="$ROOT"

die() { ui_die "$@"; }
info() { ui_info "$*"; }
ok() { ui_ok "$*"; }

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

step_github_token() {
  if github_token >/dev/null 2>&1; then
    ok "GitHub token found"
  else
    ui_warn "No GitHub token — set GITHUB_TOKEN or ~/.config/muxcore/github.token for private releases"
  fi
}

step_install_dir() {
  ui_step 1 "$TOTAL_STEPS" "Choose where MuxCore lives"
  local current="$ROOT" target
  ui_pick_directory target "Install folder" "${MUXCORE_INSTALL_DIR:-$current}"
  target="$(resolve_install_dir "$target")"
  ensure_writable_dir "$target" || die "cannot use install folder $target"
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
  ui_step 2 "$TOTAL_STEPS" "Get MuxCore components"
  echo "Downloads the core platform and media modules (auth, libraries, scanner, admin UI, …)."
  echo

  local action="Download release binaries now"
  if [[ -x "$ROOT/bin/muxcored" && -x "$ROOT/bin/api-rest" && -x "$ROOT/bin/auth-local" ]]; then
    ui_choose action \
      "Keep existing binaries in bin/" \
      "Download release binaries now" \
      --header "Components already present"
  elif [[ -d "$ROOT/../_mvp/bin" && -x "$ROOT/../_mvp/bin/muxcored" ]]; then
    ui_choose action \
      "Download release binaries now" \
      "Use developer lab binaries from _mvp/bin" \
      "Skip for now" \
      --header "How should we get MuxCore components?"
  else
    ui_choose action \
      "Download release binaries now" \
      "Skip for now" \
      --header "How should we get MuxCore components?"
  fi

  case "$action" in
    "Keep existing binaries in bin/")
      ok "keeping existing binaries"
      return 0
      ;;
    "Use developer lab binaries from _mvp/bin")
      local lab_bin
      lab_bin="$(cd "$ROOT/../_mvp/bin" && pwd)"
      export MUXCORE_LAB_BIN="$lab_bin"
      ok "lab binaries: $MUXCORE_LAB_BIN"
      ;;
    "Skip for now")
      ok "skipped — place binaries in $ROOT/bin/ then run ./install.sh"
      return 0
      ;;
  esac

  ensure_writable_dir "$ROOT/bin" || die "cannot write to $ROOT/bin (permission denied)"
  ui_spin "Downloading components (first run may take a few minutes)…" "$ROOT/install.sh"
}

step_metadata() {
  ui_step 3 "$TOTAL_STEPS" "Movie & TV metadata"
  local mode key
  ui_choose mode \
    "Offline demo library (sample titles)" \
    "TMDB API key (posters, descriptions, search)" \
    --header "MuxCore uses TMDB for rich metadata when you have a free API key"

  case "$mode" in
    "TMDB API key"*)
      ui_input_secret key "TMDB v3 API key" "${TMDB_API_KEY:-}"
      [[ -n "$key" ]] || die "TMDB API key required (get one at themoviedb.org, or pick offline demo)"
      env_set "$ENVF" TMDB_API_KEY "$key"
      env_set "$ENVF" TMDB_FIXTURE ""
      ok "live TMDB metadata enabled"
      ;;
    *)
      env_set "$ENVF" TMDB_FIXTURE 1
      env_set "$ENVF" TMDB_API_KEY ""
      ok "offline demo library enabled"
      ;;
  esac
}

step_admin() {
  ui_step 4 "$TOTAL_STEPS" "Your admin account"
  echo "Username and password for the Admin UI at http://localhost:8082"
  local user pass
  ui_input user "Admin username" "${MVP_ADMIN_USER:-admin}"
  ui_input_secret pass "Admin password (leave blank to generate)" "${MVP_ADMIN_PASSWORD:-}"
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
  ui_step 5 "$TOTAL_STEPS" "Library folders"
  echo "Pick where movies, TV shows, and incoming imports are stored."
  echo "Folders are created automatically if they do not exist."
  local movies tv incoming
  ui_pick_directory movies "Movie library folder" "${MVP_LIBRARY_ROOT:-$ROOT/data/library}"
  ui_pick_directory tv "TV library folder" "${MVP_TV_LIBRARY_ROOT:-$ROOT/data/library/tv}"
  ui_pick_directory incoming "Incoming imports folder" "${MVP_DOWNLOADS_DIR:-$ROOT/data/downloads}"
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
  ui_step 6 "$TOTAL_STEPS" "Optional extras & launch"
  load_env

  if ui_confirm "Connect an existing Jellyfin server now?" false; then
    local jf_url jf_key
    ui_input jf_url "Jellyfin URL" "${JELLYFIN_BASE_URL:-http://127.0.0.1:8096}"
    ui_input_secret jf_key "Jellyfin API key" "${JELLYFIN_API_KEY:-}"
    env_set "$ENVF" JELLYFIN_BASE_URL "$jf_url"
    env_set "$ENVF" JELLYFIN_API_KEY "$jf_key"
    ok "Jellyfin bridge configured"
  else
    ok "Jellyfin left unconfigured (configure later in Admin UI)"
  fi

  env_set "$ENVF" MUXCORE_INSECURE_DISABLE_TLS true
  env_set "$ENVF" MVP_ENABLE_MEDIA_UI 0

  if ui_confirm "Start MuxCore now?" true; then
    ui_spin "Starting MuxCore…" "$ROOT/up.sh"
    info "waiting for core health"
    local deadline=$((SECONDS + 120)) code
    until code=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/health || echo 000); [[ "$code" == "200" ]]; do
      (( SECONDS < deadline )) || die "core /health did not become ready — check $ROOT/run/core.log"
      sleep 1
    done
    ok "core is healthy"
    ui_spin "Creating your admin login…" "$ROOT/bootstrap-auth.sh"
  else
    ok "skipped start — run ./up.sh && ./bootstrap-auth.sh when ready"
  fi

  load_env
  write_view_me "$ROOT"
  ui_step 7 "$TOTAL_STEPS" "Done"
  ui_success "MuxCore is ready! Open http://localhost:8082 and sign in with the credentials below."
  ui_show_file "$ROOT/run/VIEW-ME.txt"
  if ui_confirm "Run a quick health check?" true; then
    ui_spin "Running health check…" "$ROOT/smoke-fixture.sh" || true
  fi
}

main() {
  cd "$ROOT"
  onboard_require_tty
  ui_fix_term

  prereqs_ensure_curl_first
  prereqs_ensure_gum

  ui_splash
  ui_legal_gate
  prereqs_bootstrap

  ensure_env_file
  load_env
  step_github_token
  step_install_dir "$@"
  step_fetch_binaries
  ensure_env_file
  step_metadata
  step_admin
  step_storage
  step_finish
}

main "$@"
