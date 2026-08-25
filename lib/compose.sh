# Generate a GHCR compose file for the selected (non-acquisition) modules.
# shellcheck shell=bash

compose_bin() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    echo "docker compose"
    return 0
  fi
  if command -v podman >/dev/null 2>&1 && podman compose version >/dev/null 2>&1; then
    echo "podman compose"
    return 0
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    echo "docker-compose"
    return 0
  fi
  if command -v podman-compose >/dev/null 2>&1; then
    echo "podman-compose"
    return 0
  fi
  return 1
}

compose_image() {
  local name="$1" tag
  tag="$(module_tag "$name")"
  if [[ "$name" == muxcored ]]; then
    printf 'ghcr.io/muxcore-media/muxcored:%s\n' "$tag"
    return 0
  fi
  if [[ "$name" == mediauiprox ]]; then
    printf 'ghcr.io/muxcore-media/media-ui:%s\n' "$tag"
    return 0
  fi
  printf 'ghcr.io/muxcore-media/%s:%s\n' "$name" "$tag"
}

_compose_svc() {
  local name="$1" ports="$2"
  shift 2
  module_enabled "$name" || return 0
  echo "  ${name}:"
  echo "    image: $(compose_image "$name")"
  echo "    environment:"
  echo "      MUXCORE_GRPC_ADDR: core:9090"
  echo "      MUXCORE_INSECURE_DISABLE_TLS: \"true\""
  echo "      MUXCORE_MODULE_ID: ${name}"
  echo "      MUXCORE_MESH_DIAL_LOCAL: \"true\""
  local line
  for line in "$@"; do
    echo "      ${line}"
  done
  echo "    volumes:"
  echo "      - ${COMPOSE_ROOT}/data:/data"
  if [[ "$name" == call-policy-default || "$name" == publish-policy-default ]]; then
    echo "      - ${COMPOSE_ROOT}/policies:/app/policies:ro"
  fi
  if [[ -n "$ports" ]]; then
    echo "    ports:"
    echo "$ports"
  fi
  echo "    depends_on:"
  echo "      core:"
  echo "        condition: service_healthy"
  echo "    restart: unless-stopped"
}

write_compose() {
  local root="$1"
  COMPOSE_ROOT="$root"
  local out="$root/docker-compose.yml"
  {
    cat <<EOF
name: muxcore
services:
  core:
    image: $(compose_image muxcored)
    environment:
      MUXCORE_CONFIG: /app/muxcore.json
      MUXCORE_INSECURE_DISABLE_TLS: "true"
      MUXCORE_LOG_LEVEL: info
      MUXCORE_STORAGE_DIR: /app/data/storage
    ports:
      - "8080:8080"
      - "9090:9090"
    volumes:
      - ${root}/data:/app/data
      - ${root}/muxcore.json:/app/muxcore.json:ro
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "curl -sf http://127.0.0.1:8080/health || exit 1"]
      interval: 5s
      timeout: 3s
      retries: 20
EOF
    _compose_svc api-rest "      - \"18080:8080\"" \
      "API_REST_HTTP_ADDR: \":8080\"" "API_REST_GRPC_ADDR: \":9400\""
    _compose_svc auth-local "      - \"9401:9401\"
      - \"9403:9403\"" \
      "AUTH_DB_PATH: /data/auth/auth.db" "AUTH_GRPC_ADDR: \":9403\"" "AUTH_HTTP_ADDR: \":9401\""
    _compose_svc database-sqlite "" "SQLITE_DB_PATH: /data/sqlite/muxcore.db"
    _compose_svc database-postgres "" "DATABASE_URL: ${DATABASE_URL:-}"
    _compose_svc secrets-file "" "SECRETS_STORE: /data/secrets/store.json" "SECRETS_KEY_FILE: /data/secrets/master.key"
    _compose_svc encryption-aesgcm "" "ENCRYPTION_KEY_FILE: /data/encryption/master.key"
    _compose_svc call-policy-default "" "CALL_POLICY_FILE: /app/policies/call-policy.yaml"
    _compose_svc publish-policy-default "" "PUBLISH_POLICY_FILE: /app/policies/publish-policy.yaml"
    _compose_svc cache-local "" "CACHE_LOCAL_GRPC_ADDR: \":9600\""
    _compose_svc ratelimit-tokenbucket "" "RATELIMIT_ENABLED: \"false\""
    _compose_svc health-monitor "      - \"9203:9203\"" \
      "HEALTH_MONITOR_GRPC_ADDR: \":9202\"" "HEALTH_MONITOR_HTTP_ADDR: \":9203\""
    _compose_svc admin-ui "      - \"8082:8082\"" \
      "ADMIN_UI_ADDR: \":8082\"" "ADMIN_UI_CORE_ADDR: core:9090" \
      "ADMIN_UI_INSECURE: \"true\"" "ADMIN_UI_AUTH_ADDR: http://auth-local:9401"
    _compose_svc notification-default "" "NOTIFY_GRPC_ADDR: \":9441\""
    _compose_svc metadata-tmdb "" "TMDB_API_KEY: ${TMDB_API_KEY:-}" "TMDB_FIXTURE: ${TMDB_FIXTURE:-1}"
    _compose_svc metadata-musicbrainz "" "MUSICBRAINZ_FIXTURE: ${MUSICBRAINZ_FIXTURE:-1}"
    _compose_svc media-movies "      - \"9430:9430\"" \
      "MOVIES_DB_PATH: /data/movies/movies.db" "MOVIES_IMAGE_DIR: /data/movies/images" \
      "MOVIES_HTTP_ADDR: \":9430\""
    _compose_svc media-tvshows "" \
      "TVSHOWS_DB_PATH: /data/tvshows/tvshows.db" "TVSHOWS_IMAGE_DIR: /data/tvshows/images" \
      "TVSHOWS_GRPC_ADDR: \":9440\"" "TVSHOWS_HTTP_ADDR: \":9450\""
    _compose_svc media-music "" "MUSIC_DATA_DIR: /data/music" \
      "MUSIC_LIBRARY_DIR: ${MVP_MUSIC_LIBRARY_ROOT:-/data/library/music}"
    _compose_svc media-books "" "BOOKS_DATA_DIR: /data/books"
    _compose_svc media-comics "" "COMICS_DATA_DIR: /data/comics"
    _compose_svc media-audiobooks "" "AUDIOBOOKS_DATA_DIR: /data/audiobooks"
    _compose_svc media-scanner "" \
      "SCANNER_DB_PATH: /data/scanner/scanner.db" \
      "SCANNER_LIBRARY_ROOT: ${MVP_LIBRARY_ROOT:-/data/library}" \
      "SCANNER_TV_LIBRARY_ROOT: ${MVP_TV_LIBRARY_ROOT:-/data/library/tv}" \
      "SCANNER_DEFAULT_WATCH_DIR: ${MVP_IMPORT_DIR:-/data/import}" \
      "SCANNER_IMPORT_MODE: copy"
    _compose_svc media-root-folders "" "ROOTS_DB_PATH: /data/roots/roots.db"
    _compose_svc media-rename "" "RENAME_DB_PATH: /data/rename/rename.db"
    _compose_svc media-ffprobe "" "FFPROBE_DB_PATH: /data/ffprobe/cache.db"
    _compose_svc media-subtitles "" "SUBS_DB_PATH: /data/subtitles/subtitles.db" "SUBS_DIR: /data/subtitles/files"
    _compose_svc media-custom-formats "" "FORMATS_DB_PATH: /data/formats/formats.db" "FORMATS_SEED_DEFAULTS: \"true\""
    _compose_svc media-transcoder "" "TRANSCODER_GRPC_ADDR: \":9525\"" "TRANSCODER_HTTP_ADDR: \":9526\""
    _compose_svc jellyfin "" "JELLYFIN_BASE_URL: ${JELLYFIN_BASE_URL:-}" "JELLYFIN_API_KEY: ${JELLYFIN_API_KEY:-}"
    _compose_svc plex "" "PLEX_URL: ${PLEX_URL:-}" "PLEX_TOKEN: ${PLEX_TOKEN:-}"
    _compose_svc emby "" "EMBY_URL: ${EMBY_URL:-}" "EMBY_TOKEN: ${EMBY_TOKEN:-}"
    _compose_svc media-dlna "" "DLNA_MEDIA_PATH: ${DLNA_MEDIA_PATH:-${MVP_LIBRARY_ROOT:-/data/library}}"
  } >"$out"
  echo "wrote $out"
}

compose_login_ghcr() {
  local token
  token="$(github_token 2>/dev/null || true)"
  [[ -n "$token" ]] || return 1
  if command -v docker >/dev/null 2>&1; then
    echo "$token" | docker login ghcr.io -u TOKEN --password-stdin >/dev/null 2>&1 && return 0
  fi
  if command -v podman >/dev/null 2>&1; then
    echo "$token" | podman login ghcr.io -u TOKEN --password-stdin >/dev/null 2>&1 && return 0
  fi
  return 1
}

compose_up() {
  local root="$1"
  local bin
  bin="$(compose_bin)" || ui_die "Docker Compose or Podman Compose is required for this path"
  write_compose "$root"
  echo "==> pulling images from ghcr.io/muxcore-media"
  # shellcheck disable=SC2086
  if ! (cd "$root" && $bin pull); then
    echo "==> pull failed — images may still be private"
    if github_token >/dev/null 2>&1 || github_prompt_token; then
      compose_login_ghcr || true
      # shellcheck disable=SC2086
      if ! (cd "$root" && $bin pull); then
        return 1
      fi
    else
      return 1
    fi
  fi
  # shellcheck disable=SC2086
  (cd "$root" && $bin up -d)
}
