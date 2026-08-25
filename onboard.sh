#!/usr/bin/env bash
# Interactive first-run walkthrough for MuxCore.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
ENVF="$ROOT/.env"
EXAMPLE="$ROOT/.env.example"
TOTAL_STEPS=10

# shellcheck disable=SC1091
source "$ROOT/lib/common.sh"
# shellcheck disable=SC1091
source "$ROOT/lib/github.sh"
# shellcheck disable=SC1091
source "$ROOT/lib/ui.sh"
# shellcheck disable=SC1091
source "$ROOT/lib/prereqs.sh"
# shellcheck disable=SC1091
source "$ROOT/lib/paths.sh"
# shellcheck disable=SC1091
source "$ROOT/lib/modules.sh"
# shellcheck disable=SC1091
source "$ROOT/lib/seed-roots.sh"
# shellcheck disable=SC1091
source "$ROOT/lib/compose.sh"
# shellcheck disable=SC1091
source "$ROOT/versions.env"

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
  fi
}

copy_installer_to() {
  local target="$1"
  if command -v rsync >/dev/null 2>&1; then
    rsync -a --exclude data --exclude run --exclude bin --exclude cache \
      "$ROOT/" "$target/"
  else
    mkdir -p "$target"
    tar -C "$ROOT" --exclude=data --exclude=run --exclude=bin --exclude=cache -cf - . \
      | tar -C "$target" -xf -
  fi
}

step_install_dir() {
  ui_step 1 "$TOTAL_STEPS" "Install folder"
  target=""
  if ui_noninteractive || [[ "${MUXCORE_DRY_RUN:-}" == 1 ]]; then
    ok "using $ROOT"
    return 0
  fi
  ui_pick_dir target "Where should MuxCore be installed?" \
    "$(paths_install_default)" "$(paths_install_recommended)"
  target="$(resolve_install_dir "$target")"
  ensure_writable_dir "$target" || die "cannot use install folder $target"
  if [[ "$(cd "$target" && pwd)" != "$ROOT" ]]; then
    info "copying installer into $target"
    copy_installer_to "$target"
    ok "continuing in $target"
    exec bash "$target/onboard.sh" "$@"
  fi
  ok "using $ROOT"
}

step_runtime() {
  ui_step 2 "$TOTAL_STEPS" "How to run MuxCore"
  choice=""
  ui_choose choice \
    "Host processes (no Docker required)" \
    "Docker Compose (needs Docker or Podman)" \
    --header "Host processes download binaries and start them on this machine.
Compose pulls images from GitHub Container Registry."
  case "$choice" in
    "Docker Compose"*)
      INSTALL_RUNTIME=compose
      prereqs_offer_docker
      if ! prereqs_have_compose && ! command -v docker >/dev/null 2>&1; then
        ui_warn "Compose tools not found — switching to host processes"
        INSTALL_RUNTIME=host
      fi
      ;;
    *)
      INSTALL_RUNTIME=host
      ;;
  esac
  env_set "$ENVF" INSTALL_RUNTIME "$INSTALL_RUNTIME"
  ok "runtime: $INSTALL_RUNTIME"
}

step_libraries() {
  ui_step 3 "$TOTAL_STEPS" "Libraries"
  echo "MuxCore organizes files you already have. Select the kinds you keep."
  ui_multi_list LIBRARIES "Movies,TV" \
    Movies TV Music Books Comics Audiobooks
  [[ -n "$LIBRARIES" ]] || LIBRARIES="Movies,TV"
  env_set "$ENVF" MUXCORE_LIBRARIES "$LIBRARIES"
  ok "libraries: $LIBRARIES"
}

step_media_paths() {
  ui_step 4 "$TOTAL_STEPS" "Media folders"
  root_media=""
  ui_pick_dir root_media "Where should your media live?" \
    "$(paths_media_default)" "$(paths_media_recommended)"
  root_media="$(resolve_install_dir "$root_media")"
  mkdir -p "$root_media"

  movies=""; tv=""; music=""; books=""; comics=""; audio=""
  csv_has "$LIBRARIES" Movies && paths_confirm_kind movies Movies "$root_media/Movies"
  csv_has "$LIBRARIES" TV && paths_confirm_kind tv TV "$root_media/TV"
  csv_has "$LIBRARIES" Music && paths_confirm_kind music Music "$root_media/Music"
  csv_has "$LIBRARIES" Books && paths_confirm_kind books Books "$root_media/Books"
  csv_has "$LIBRARIES" Comics && paths_confirm_kind comics Comics "$root_media/Comics"
  csv_has "$LIBRARIES" Audiobooks && paths_confirm_kind audio Audiobooks "$root_media/Audiobooks"

  # TV must not nest under the movie root.
  if [[ -n "$movies" && -n "$tv" ]]; then
    case "$tv" in
      "$movies"|"$movies"/*)
        ui_warn "TV folder cannot sit inside the movie folder — using $root_media/TV"
        tv="$root_media/TV"
        ;;
    esac
  fi

  incoming=""
  if ui_confirm "Also watch a folder for files you copy in yourself?" true; then
    ui_pick_dir incoming "Import / watch folder" \
      "$root_media/Import" "$HOME/Downloads"
    incoming="$(resolve_install_dir "$incoming")"
  else
    incoming="$root_media/Import"
  fi

  [[ -n "$movies" ]] && mkdir -p "$movies" && env_set "$ENVF" MVP_LIBRARY_ROOT "$movies"
  [[ -n "$tv" ]] && mkdir -p "$tv" && env_set "$ENVF" MVP_TV_LIBRARY_ROOT "$tv"
  [[ -n "$music" ]] && mkdir -p "$music" && env_set "$ENVF" MVP_MUSIC_LIBRARY_ROOT "$music"
  [[ -n "$books" ]] && mkdir -p "$books" && env_set "$ENVF" MVP_BOOKS_LIBRARY_ROOT "$books"
  [[ -n "$comics" ]] && mkdir -p "$comics" && env_set "$ENVF" MVP_COMICS_LIBRARY_ROOT "$comics"
  [[ -n "$audio" ]] && mkdir -p "$audio" && env_set "$ENVF" MVP_AUDIOBOOKS_LIBRARY_ROOT "$audio"
  mkdir -p "$incoming"
  env_set "$ENVF" MVP_IMPORT_DIR "$incoming"
  env_set "$ENVF" MVP_DOWNLOADS_DIR "$incoming"
  env_set "$ENVF" MUXCORE_STORAGE_DIR "${MUXCORE_STORAGE_DIR:-$ROOT/data/storage}"
  ok "media root → $root_media"
}

step_playback() {
  ui_step 5 "$TOTAL_STEPS" "Watch and play"
  echo "MuxCore can play files itself, or talk to a media server you already run."
  ui_multi_list PLAYBACK "" \
    "MuxCore player" Jellyfin Plex Emby DLNA
  env_set "$ENVF" MUXCORE_PLAYBACK "$PLAYBACK"
  env_set "$ENVF" MVP_ENABLE_MEDIA_UI 0

  if csv_has "$PLAYBACK" "MuxCore player"; then
    env_set "$ENVF" MVP_ENABLE_MEDIA_UI 1
    prereqs_ensure_ffmpeg
  fi
  if csv_has "$PLAYBACK" Jellyfin; then
    jf_url=""; jf_key=""
    ui_input jf_url "Existing Jellyfin URL" "${JELLYFIN_BASE_URL:-http://127.0.0.1:8096}"
    ui_password jf_key "Jellyfin API key" "${JELLYFIN_API_KEY:-}"
    if [[ -n "$jf_url" && -n "$jf_key" ]]; then
      env_set "$ENVF" JELLYFIN_BASE_URL "$jf_url"
      env_set "$ENVF" JELLYFIN_API_KEY "$jf_key"
    else
      ui_warn "Jellyfin skipped — URL and API key are both required"
      PLAYBACK="$(printf '%s' "$PLAYBACK" | sed 's/Jellyfin//;s/,,/,/g')"
    fi
  fi
  if csv_has "$PLAYBACK" Plex; then
    plex_url=""; plex_tok=""
    ui_input plex_url "Existing Plex URL" "${PLEX_URL:-http://127.0.0.1:32400}"
    ui_password plex_tok "Plex token" "${PLEX_TOKEN:-}"
    if [[ -n "$plex_url" && -n "$plex_tok" ]]; then
      env_set "$ENVF" PLEX_URL "$plex_url"
      env_set "$ENVF" PLEX_TOKEN "$plex_tok"
    else
      ui_warn "Plex skipped — URL and token are both required"
    fi
  fi
  if csv_has "$PLAYBACK" Emby; then
    emby_url=""; emby_tok=""
    ui_input emby_url "Existing Emby URL" "${EMBY_URL:-http://127.0.0.1:8096}"
    ui_password emby_tok "Emby token" "${EMBY_TOKEN:-}"
    if [[ -n "$emby_url" && -n "$emby_tok" ]]; then
      env_set "$ENVF" EMBY_URL "$emby_url"
      env_set "$ENVF" EMBY_TOKEN "$emby_tok"
    else
      ui_warn "Emby skipped — URL and token are both required"
    fi
  fi
  if csv_has "$PLAYBACK" DLNA; then
    env_set "$ENVF" DLNA_MEDIA_PATH "${MVP_LIBRARY_ROOT:-$ROOT/data/library}"
  fi
}

step_admin() {
  ui_step 6 "$TOTAL_STEPS" "First admin login"
  user=""; pass=""
  ui_input user "Admin username" "${MVP_ADMIN_USER:-admin}"
  ui_password pass "Admin password (leave blank to generate)" ""
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

step_metadata() {
  ui_step 7 "$TOTAL_STEPS" "Metadata"
  mode=""; key=""
  if csv_has "$LIBRARIES" Movies || csv_has "$LIBRARIES" TV; then
    ui_choose mode \
      "Offline sample titles (no API key)" \
      "TMDB API key (posters and descriptions)" \
      --header "Movie and TV metadata"
    case "$mode" in
      "TMDB API key"*)
        ui_password key "TMDB v3 API key" "${TMDB_API_KEY:-}"
        [[ -n "$key" ]] || die "TMDB API key required, or pick offline samples"
        env_set "$ENVF" TMDB_API_KEY "$key"
        env_set "$ENVF" TMDB_FIXTURE ""
        ;;
      *)
        env_set "$ENVF" TMDB_FIXTURE 1
        env_set "$ENVF" TMDB_API_KEY ""
        ;;
    esac
  fi
  if csv_has "$LIBRARIES" Music; then
    env_set "$ENVF" MUSICBRAINZ_FIXTURE 1
    ok "MusicBrainz offline fixture enabled (no key required)"
  fi
}

step_database() {
  ui_step 8 "$TOTAL_STEPS" "Database"
  choice=""
  ui_choose choice \
    "SQLite (recommended for a single machine)" \
    "Postgres (existing URL, or a local Docker database)" \
    --header "Where should MuxCore keep its own data?"
  case "$choice" in
    Postgres*)
      MUXCORE_PROFILE=postgres
      env_set "$ENVF" MUXCORE_PROFILE postgres
      url=""
      ui_input url "DATABASE_URL (leave blank to use Docker postgres:16-alpine)" \
        "${DATABASE_URL:-}"
      if [[ -n "$url" ]]; then
        env_set "$ENVF" DATABASE_URL "$url"
      else
        prereqs_offer_docker
      fi
      ;;
    *)
      MUXCORE_PROFILE=sqlite
      env_set "$ENVF" MUXCORE_PROFILE sqlite
      ;;
  esac
}

step_keepalive() {
  ui_step 9 "$TOTAL_STEPS" "Keep running"
  KEEP_MODE=once
  if [[ "${INSTALL_RUNTIME:-host}" != host ]]; then
    ok "Compose will restart containers unless stopped"
    KEEP_MODE=compose
    env_set "$ENVF" MUXCORE_KEEP_MODE "$KEEP_MODE"
    return 0
  fi
  choice=""
  ui_choose choice \
    "Start now with ./up.sh (stops when you run ./up.sh stop)" \
    "Also install a user systemd service" \
    "Also install a system systemd service (needs sudo)" \
    --header "How should MuxCore stay running?"
  case "$choice" in
    *"user systemd"*)
      KEEP_MODE=user
      ;;
    *"system systemd"*)
      KEEP_MODE=system
      ;;
    *)
      KEEP_MODE=once
      ;;
  esac
  env_set "$ENVF" MUXCORE_KEEP_MODE "$KEEP_MODE"
}

write_systemd_user() {
  local unit="$HOME/.config/systemd/user/muxcore.service"
  mkdir -p "$(dirname "$unit")"
  cat >"$unit" <<EOF
[Unit]
Description=MuxCore host stack
After=network-online.target

[Service]
Type=forking
WorkingDirectory=$ROOT
ExecStart=$ROOT/up.sh
ExecStop=$ROOT/up.sh stop
Restart=on-failure

[Install]
WantedBy=default.target
EOF
  if command -v systemctl >/dev/null 2>&1; then
    systemctl --user daemon-reload || true
    systemctl --user enable --now muxcore.service || "$ROOT/up.sh"
  else
    "$ROOT/up.sh"
  fi
}

write_systemd_system() {
  local unit=/etc/systemd/system/muxcore.service
  local tmp
  tmp="$(mktemp)"
  cat >"$tmp" <<EOF
[Unit]
Description=MuxCore host stack
After=network-online.target

[Service]
Type=forking
User=$(id -un)
WorkingDirectory=$ROOT
ExecStart=$ROOT/up.sh
ExecStop=$ROOT/up.sh stop
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF
  if command -v sudo >/dev/null 2>&1; then
    sudo install -m 0644 "$tmp" "$unit"
    sudo systemctl daemon-reload
    sudo systemctl enable --now muxcore.service
  else
    ui_warn "no sudo — starting with ./up.sh instead"
    "$ROOT/up.sh"
  fi
  rm -f "$tmp"
}

step_summary_and_go() {
  ui_step 10 "$TOTAL_STEPS" "Summary"
  load_env
  ENABLED_MODULES="$(resolve_enabled_modules "${LIBRARIES:-Movies,TV}" "${PLAYBACK:-}" "${MUXCORE_PROFILE:-sqlite}")"
  export ENABLED_MODULES
  env_set "$ENVF" ENABLED_MODULES "$ENABLED_MODULES"
  env_set "$ENVF" MUXCORE_INSECURE_DISABLE_TLS true

  cat <<EOF

Install folder:  $ROOT
Runtime:         ${INSTALL_RUNTIME:-host}
Libraries:       ${LIBRARIES:-}
Playback:        ${PLAYBACK:-none}
Database:        ${MUXCORE_PROFILE:-sqlite}
Admin user:      ${MVP_ADMIN_USER:-admin}
Modules:         $ENABLED_MODULES

EOF
  if [[ "${MUXCORE_DRY_RUN:-}" == 1 ]]; then
    ok "dry run — stopping before download and start"
    write_view_me "$ROOT"
    return 0
  fi
  if ! ui_confirm "Download components and start MuxCore now?" true; then
    ok "stopped before install — run ./install.sh && ./up.sh later"
    write_view_me "$ROOT"
    return 0
  fi

  ui_spin "Fetching MuxCore components…" "$ROOT/install.sh"
  seed_wizard_roots "$ROOT"

  if [[ "${INSTALL_RUNTIME:-host}" == compose ]]; then
    if ! compose_up "$ROOT"; then
      ui_warn "Compose pull/start failed — falling back to host processes"
      INSTALL_RUNTIME=host
      env_set "$ENVF" INSTALL_RUNTIME host
      ui_spin "Starting MuxCore…" "$ROOT/up.sh"
    fi
  else
    case "${KEEP_MODE:-once}" in
      user) write_systemd_user ;;
      system) write_systemd_system ;;
      *) ui_spin "Starting MuxCore…" "$ROOT/up.sh" ;;
    esac
  fi

  info "waiting for core health"
  local deadline=$((SECONDS + 120)) code
  until code=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/health || echo 000); [[ "$code" == "200" || "$code" == "503" ]]; do
    if [[ "$SECONDS" -ge "$deadline" ]]; then
      die "core /health did not become ready — check $ROOT/run/core.log"
    fi
    sleep 1
  done
  ok "core is up"
  ui_spin "Creating your admin login…" "$ROOT/bootstrap-auth.sh" || ui_warn "admin bootstrap failed (auth may still be starting)"
  load_env
  write_view_me "$ROOT"
  ui_success "MuxCore is ready. Open http://localhost:8082 and sign in with the credentials in run/VIEW-ME.txt."
  ui_show_file "$ROOT/run/VIEW-ME.txt"
  if ui_confirm "Run a quick health check?" true; then
    ui_spin "Running health check…" "$ROOT/smoke-fixture.sh" || true
  fi
}

main() {
  cd "$ROOT"
  onboard_require_tty
  ui_fix_term
  prereqs_refuse_root

  echo "Preparing the installer…"
  prereqs_ensure_curl_first
  prereqs_ensure_gum
  UI_BACKEND=""
  ui_detect_backend

  ui_splash
  ui_legal_gate
  prereqs_bootstrap
  trap prereqs_sudo_k EXIT

  ensure_env_file
  load_env
  ui_record_legal "$ROOT"

  step_install_dir "$@"
  ensure_env_file
  step_runtime
  step_libraries
  step_media_paths
  step_playback
  step_admin
  step_metadata
  step_database
  step_keepalive
  step_summary_and_go
}

main "$@"
