#!/usr/bin/env bash
# Start/stop the MuxCore host stack from installer bin/ (release or lab binaries).
# Usage: ./up.sh | ./up.sh stop
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
BIN="$ROOT/bin"
RUN="$ROOT/run"
DATA="$ROOT/data"

# shellcheck disable=SC1091
[[ -f "$ROOT/.env" ]] || {
  if [[ -f "$ROOT/.env.example" ]]; then
    echo "==> no .env; copying .env.example (fixture defaults)"
    cp "$ROOT/.env.example" "$ROOT/.env"
  else
    echo "FAIL: missing $ROOT/.env (and no .env.example) — run ./install.sh first" >&2
    exit 1
  fi
}
# shellcheck disable=SC1091
source "$ROOT/.env" || true
# shellcheck disable=SC1091
source "$ROOT/versions.env"

export MUXCORE_INSECURE_DISABLE_TLS=true
export MUXCORE_LOG_LEVEL="${MUXCORE_LOG_LEVEL:-info}"
export MUXCORE_CONFIG="${MUXCORE_CONFIG:-$ROOT/muxcore.json}"
MESH="${MUXCORE_MESH_ADDR:-127.0.0.1:9090}"

mkdir -p "$BIN" "$RUN" \
  "$DATA"/{movies,tvshows,automation,scanner,roots,sqlite,secrets,encryption,library/tv,storage,auth,jellyfin,downloads,request,formats,rename,ffprobe,subtitles/files}

have_bin() { [[ -x "$BIN/$1" ]]; }

start_one() {
  local name="$1"; shift
  local pidfile="$RUN/$name.pid"
  local logfile="$RUN/$name.log"
  if [[ -f "$pidfile" ]] && kill -0 "$(cat "$pidfile")" 2>/dev/null; then
    echo "already running $name (pid $(cat "$pidfile"))"
    return 0
  fi
  echo "starting $name -> $logfile"
  nohup "$@" >"$logfile" 2>&1 &
  echo $! >"$pidfile"
}

# start_mod NAME BIN_NAME ENV...  — skips with WARN when binary missing
start_mod() {
  local name="$1" bin_name="$2"; shift 2
  if ! have_bin "$bin_name"; then
    echo "WARN: skip $name — missing bin/$bin_name (re-run ./install.sh or set MUXCORE_LAB_BIN)" >&2
    return 0
  fi
  start_one "$name" env "$@" "$BIN/$bin_name"
}

stop_all() {
  local f pid name
  for f in "$RUN"/*.pid; do
    [[ -f "$f" ]] || continue
    pid=$(cat "$f")
    name=$(basename "$f" .pid)
    if kill -0 "$pid" 2>/dev/null; then
      echo "stopping $name ($pid)"
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
    rm -f "$f"
  done
}

# Optional profile: sqlite (default) | postgres
PROFILE="${MUXCORE_PROFILE:-${INSTALLER_PROFILE:-sqlite}}"

ensure_postgres() {
  # When PROFILE=postgres, prefer an existing DATABASE_URL / PG* env; otherwise
  # start a local Docker Postgres if Docker is available.
  if [[ -n "${DATABASE_URL:-}" || -n "${PGHOST:-}" ]]; then
    echo "==> postgres profile: using existing DATABASE_URL/PG* env"
    return 0
  fi
  if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
    cat >&2 <<EOF
FAIL: MUXCORE_PROFILE=postgres requires either:
  - DATABASE_URL (or PGHOST/PGUSER/PGPASSWORD/PGDATABASE), or
  - Docker to start postgres:16-alpine on 127.0.0.1:5432
EOF
    return 1
  fi
  local name="${MUXCORE_PG_CONTAINER:-muxcore-installer-pg}"
  if docker ps --format '{{.Names}}' | grep -qx "$name"; then
    echo "==> postgres already running ($name)"
  elif docker ps -a --format '{{.Names}}' | grep -qx "$name"; then
    echo "==> starting existing postgres container $name"
    docker start "$name" >/dev/null
  else
    echo "==> docker run postgres:16-alpine ($name) on 127.0.0.1:5432"
    docker run -d --name "$name" \
      -e POSTGRES_USER=muxcore \
      -e POSTGRES_PASSWORD=muxcore \
      -e POSTGRES_DB=muxcore \
      -p 127.0.0.1:5432:5432 \
      postgres:16-alpine >/dev/null
  fi
  export PGHOST=127.0.0.1 PGPORT=5432 PGUSER=muxcore PGPASSWORD=muxcore PGDATABASE=muxcore PGSSLMODE=disable
  export DATABASE_URL="postgres://muxcore:muxcore@127.0.0.1:5432/muxcore?sslmode=disable"
  # Wait for accept
  for _ in $(seq 1 40); do
    if docker exec "$name" pg_isready -U muxcore >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  echo "WARN: postgres may not be ready yet; continuing" >&2
  return 0
}

usage() {
  cat <<'EOF'
usage: ./up.sh [stop]

  (default)  Start host stack from bin/ (default platform + media modules)
  stop       Stop all processes tracked in run/*.pid

Optional env:
  MUXCORE_PROFILE=sqlite|postgres   (default sqlite)
    postgres — skip database-sqlite; start database-postgres; start local
               Docker Postgres if DATABASE_URL/PG* unset (skips hard-fail
               only when Docker unavailable and no DATABASE_URL)
  MUXCORE_OBSERVABILITY=1 — also start metrics-prometheus + tracing-otlp when
               binaries are present (health-monitor remains on by default)

Requires bin/muxcored (from ./install.sh or MUXCORE_LAB_BIN). Modules listed in
versions.env are started when their binary is present; missing modules are skipped
with a warning (except muxcored, which fails hard).
EOF
}

cmd="${1:-up}"
case "$cmd" in
  -h|--help) usage; exit 0 ;;
  stop) stop_all; exit 0 ;;
  up|"")
    ;;
  *)
    echo "unknown arg: $cmd" >&2
    usage >&2
    exit 2
    ;;
esac

if ! have_bin muxcored; then
  cat >&2 <<EOF
FAIL: missing bin/muxcored

Run ./install.sh to download release assets (or copy lab binaries), then re-run.
Lab fallback: export MUXCORE_LAB_BIN=/path/to/_mvp/bin && ./install.sh

This installer does not build the monorepo for you.
EOF
  exit 1
fi

stop_all

echo "==> host stack (profile=${PROFILE}; default + media modules)"

if [[ "$PROFILE" == "postgres" ]]; then
  ensure_postgres
elif [[ "$PROFILE" != "sqlite" ]]; then
  echo "FAIL: unknown MUXCORE_PROFILE=$PROFILE (want sqlite|postgres)" >&2
  exit 2
fi

start_one core env \
  MUXCORE_CONFIG="$ROOT/muxcore.json" \
  MUXCORE_INSECURE_DISABLE_TLS=true \
  MUXCORE_STORAGE_DIR="${MUXCORE_STORAGE_DIR:-$DATA/storage}" \
  MUXCORE_LOG_LEVEL="${MUXCORE_LOG_LEVEL:-info}" \
  "$BIN/muxcored"

for _ in $(seq 1 40); do
  code=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/health || echo 000)
  [[ "$code" == "200" || "$code" == "503" ]] && break
  sleep 0.5
done

C=(MUXCORE_GRPC_ADDR="$MESH" MUXCORE_INSECURE_DISABLE_TLS=true)

start_mod api-rest api-rest \
  "${C[@]}" MUXCORE_MODULE_ID=api-rest \
  API_REST_HTTP_ADDR="${API_REST_HTTP_ADDR:-:18080}" \
  API_REST_GRPC_ADDR="${API_REST_GRPC_ADDR:-:9400}"

start_mod auth-local auth-local \
  "${C[@]}" MUXCORE_MODULE_ID=auth-local \
  AUTH_DB_PATH="$DATA/auth/auth.db" \
  AUTH_GRPC_ADDR=":9403" AUTH_HTTP_ADDR=":9401"

if [[ "$PROFILE" == "postgres" ]]; then
  start_mod database-postgres database-postgres \
    "${C[@]}" MUXCORE_MODULE_ID=database-postgres \
    DATABASE_GRPC_ADDR="${DATABASE_GRPC_ADDR:-:9701}" \
    DATABASE_URL="${DATABASE_URL:-}" \
    PGHOST="${PGHOST:-127.0.0.1}" \
    PGPORT="${PGPORT:-5432}" \
    PGUSER="${PGUSER:-muxcore}" \
    PGPASSWORD="${PGPASSWORD:-muxcore}" \
    PGDATABASE="${PGDATABASE:-muxcore}" \
    PGSSLMODE="${PGSSLMODE:-disable}"
else
  start_mod database-sqlite database-sqlite \
    "${C[@]}" MUXCORE_MODULE_ID=database-sqlite \
    SQLITE_DB_PATH="$DATA/sqlite/muxcore.db"
fi

start_mod secrets-file secrets-file \
  "${C[@]}" MUXCORE_MODULE_ID=secrets-file \
  SECRETS_STORE="$DATA/secrets/store.json" \
  SECRETS_KEY_FILE="$DATA/secrets/master.key"

start_mod encryption-aesgcm encryption-aesgcm \
  "${C[@]}" MUXCORE_MODULE_ID=encryption-aesgcm \
  ENCRYPTION_KEY_FILE="$DATA/encryption/master.key"

CALL_POLICY="${CALL_POLICY_FILE:-$ROOT/policies/call-policy.yaml}"
PUBLISH_POLICY="${PUBLISH_POLICY_FILE:-$ROOT/policies/publish-policy.yaml}"

start_mod call-policy-default call-policy-default \
  "${C[@]}" MUXCORE_MODULE_ID=call-policy-default \
  CALL_POLICY_FILE="$CALL_POLICY"

start_mod publish-policy-default publish-policy-default \
  "${C[@]}" MUXCORE_MODULE_ID=publish-policy-default \
  PUBLISH_POLICY_FILE="$PUBLISH_POLICY"

start_mod cache-local cache-local \
  "${C[@]}" MUXCORE_MODULE_ID=cache-local \
  CACHE_LOCAL_GRPC_ADDR="${CACHE_LOCAL_GRPC_ADDR:-:9600}"

start_mod ratelimit-tokenbucket ratelimit-tokenbucket \
  "${C[@]}" MUXCORE_MODULE_ID=ratelimit-tokenbucket \
  RATELIMIT_ENABLED="${RATELIMIT_ENABLED:-false}" \
  RATELIMIT_RATE="${RATELIMIT_RATE:-100}" \
  RATELIMIT_BURST="${RATELIMIT_BURST:-200}"

start_mod health-monitor health-monitor \
  "${C[@]}" MUXCORE_MODULE_ID=health-monitor \
  MUXCORE_MESH_DIAL_LOCAL=true \
  HEALTH_MONITOR_GRPC_ADDR="${HEALTH_MONITOR_GRPC_ADDR:-:9202}" \
  HEALTH_MONITOR_HTTP_ADDR="${HEALTH_MONITOR_HTTP_ADDR:-:9203}" \
  HEALTH_MONITOR_INTERVAL="${HEALTH_MONITOR_INTERVAL:-5s}"

# Optional metrics/tracing (MUXCORE_OBSERVABILITY=1)
if [[ "${MUXCORE_OBSERVABILITY:-0}" == "1" || "${MUXCORE_OBSERVABILITY:-}" == "true" ]]; then
  start_mod metrics-prometheus metrics-prometheus \
    "${C[@]}" MUXCORE_MODULE_ID=metrics-prometheus \
    METRICS_GRPC_ADDR="${METRICS_GRPC_ADDR:-:9900}" \
    METRICS_HTTP_ADDR="${METRICS_HTTP_ADDR:-:9901}" \
    METRICS_PATH="${METRICS_PATH:-/metrics}"
  start_mod tracing-otlp tracing-otlp \
    "${C[@]}" MUXCORE_MODULE_ID=tracing-otlp \
    TRACING_GRPC_ADDR="${TRACING_GRPC_ADDR:-:9613}" \
    OTEL_EXPORTER_OTLP_ENDPOINT="${OTEL_EXPORTER_OTLP_ENDPOINT:-}" \
    OTEL_EXPORTER_OTLP_INSECURE="${OTEL_EXPORTER_OTLP_INSECURE:-true}"
fi

start_mod admin-ui admin-ui \
  ADMIN_UI_ADDR=":8082" \
  ADMIN_UI_CORE_ADDR="$MESH" \
  ADMIN_UI_INSECURE=true \
  ADMIN_UI_AUTH_ADDR="http://127.0.0.1:9401" \
  ADMIN_UI_HEALTH_MONITOR_URL="${ADMIN_UI_HEALTH_MONITOR_URL:-http://127.0.0.1:9203}" \
  MUXCORE_MESH_DIAL_LOCAL=true

start_mod metadata-tmdb metadata-tmdb \
  "${C[@]}" MUXCORE_MODULE_ID=metadata-tmdb \
  TMDB_API_KEY="${TMDB_API_KEY:-}" \
  MUXCORE_CFG_TMDB_API_KEY="${MUXCORE_CFG_TMDB_API_KEY:-${TMDB_API_KEY:-}}" \
  TMDB_FIXTURE="${TMDB_FIXTURE:-1}"

start_mod media-movies media-movies \
  "${C[@]}" MUXCORE_MODULE_ID=media-movies \
  MOVIES_DB_PATH="$DATA/movies/movies.db" MOVIES_IMAGE_DIR="$DATA/movies/images" \
  MOVIES_HTTP_ADDR=":9430"

start_mod media-tvshows media-tvshows \
  "${C[@]}" MUXCORE_MODULE_ID=media-tvshows \
  TVSHOWS_DB_PATH="$DATA/tvshows/tvshows.db" TVSHOWS_IMAGE_DIR="$DATA/tvshows/images" \
  TVSHOWS_GRPC_ADDR=":9440" TVSHOWS_HTTP_ADDR=":9450"

start_mod media-automation media-automation \
  "${C[@]}" MUXCORE_MODULE_ID=media-automation \
  MUXCORE_MESH_DIAL_LOCAL=true \
  AUTOMATION_DB_PATH="$DATA/automation/automation.db" \
  AUTOMATION_GRPC_ADDR=":9460" \
  AUTOMATION_EVENT_SUBSCRIBE_DELAY=1s \
  AUTOMATION_DOWNLOAD_DIR="${MVP_DOWNLOADS_DIR:-$DATA/downloads}"

LIBRARY_ROOT="${MVP_LIBRARY_ROOT:-$DATA/library}"
TV_LIBRARY_ROOT="${MVP_TV_LIBRARY_ROOT:-$DATA/library/tv}"
INCOMING_DIR="${MVP_DOWNLOADS_DIR:-$DATA/downloads}"
mkdir -p "$LIBRARY_ROOT" "$TV_LIBRARY_ROOT" "$INCOMING_DIR" \
  "$DATA/formats" "$DATA/rename" "$DATA/ffprobe" "$DATA/subtitles/files"

start_mod media-custom-formats media-custom-formats \
  "${C[@]}" MUXCORE_MODULE_ID=media-custom-formats \
  FORMATS_DB_PATH="$DATA/formats/formats.db" \
  FORMATS_GRPC_ADDR=":9490" \
  FORMATS_SEED_DEFAULTS=true

start_mod media-rename media-rename \
  "${C[@]}" MUXCORE_MODULE_ID=media-rename \
  RENAME_DB_PATH="$DATA/rename/rename.db" \
  RENAME_GRPC_ADDR="${RENAME_GRPC_ADDR:-:9510}" \
  RENAME_IMPORT_MODE=copy

start_mod media-ffprobe media-ffprobe \
  "${C[@]}" MUXCORE_MODULE_ID=media-ffprobe \
  FFPROBE_DB_PATH="$DATA/ffprobe/cache.db" \
  FFPROBE_GRPC_ADDR="${FFPROBE_GRPC_ADDR:-:9480}"

start_mod media-subtitles media-subtitles \
  "${C[@]}" MUXCORE_MODULE_ID=media-subtitles \
  SUBS_DB_PATH="$DATA/subtitles/subtitles.db" \
  SUBS_DIR="$DATA/subtitles/files" \
  SUBS_GRPC_ADDR="${SUBS_GRPC_ADDR:-:9520}"

start_mod media-scanner media-scanner \
  "${C[@]}" MUXCORE_MODULE_ID=media-scanner \
  SCANNER_DB_PATH="$DATA/scanner/scanner.db" \
  SCANNER_LIBRARY_ROOT="$LIBRARY_ROOT" \
  SCANNER_TV_LIBRARY_ROOT="$TV_LIBRARY_ROOT" \
  SCANNER_DEFAULT_WATCH_DIR="$INCOMING_DIR" \
  SCANNER_GRPC_ADDR=":9470" \
  SCANNER_IMPORT_MODE=copy \
  SCANNER_MIN_VIDEO_BYTES=0

start_mod media-root-folders media-root-folders \
  "${C[@]}" MUXCORE_MODULE_ID=media-root-folders \
  ROOTS_DB_PATH="$DATA/roots/roots.db"

start_mod request-media request-media \
  "${C[@]}" MUXCORE_MODULE_ID=request-media \
  MUXCORE_MESH_DIAL_LOCAL=true \
  REQUEST_GRPC_ADDR=":9481" \
  REQUEST_HTTP_ADDR=":9380" \
  REQUEST_DATA_DIR="$DATA/request"

start_mod notification-default notification-default \
  "${C[@]}" MUXCORE_MODULE_ID=notification-default \
  NOTIFY_GRPC_ADDR=":9441" \
  WEBHOOK_URL="${NOTIFY_WEBHOOK_URL:-http://127.0.0.1:9/muxcore-notify-sink}"

start_mod jellyfin jellyfin \
  "${C[@]}" MUXCORE_MODULE_ID=jellyfin \
  JELLYFIN_GRPC_ADDR=":9475" JELLYFIN_HTTP_ADDR=":8475" \
  JELLYFIN_DATA_DIR="$DATA/jellyfin" \
  JELLYFIN_BASE_URL="${JELLYFIN_BASE_URL:-}" \
  JELLYFIN_API_KEY="${JELLYFIN_API_KEY:-}" \
  JELLYFIN_WEBHOOK_SECRET="${JELLYFIN_WEBHOOK_SECRET:-}"

# Optional consumer SPA (off by default in .env.example).
if [[ "${MVP_ENABLE_MEDIA_UI:-0}" != "0" ]]; then
  UI_DIST="${MEDIA_UI_DIST:-}"
  if [[ -z "$UI_DIST" || ! -d "$UI_DIST" ]]; then
    echo "WARN: MVP_ENABLE_MEDIA_UI set but MEDIA_UI_DIST missing — skipping media-ui" >&2
  elif have_bin mediauiprox; then
    start_one media-ui env \
      MEDIA_UI_LISTEN="${MEDIA_UI_LISTEN:-:5173}" \
      MEDIA_UI_DIST="$UI_DIST" \
      MEDIA_UI_REQUIRE_AUTH="${MEDIA_UI_REQUIRE_AUTH:-1}" \
      AUTH_HTTP_URL="${AUTH_HTTP_URL:-http://127.0.0.1:9401}" \
      MOVIES_GRPC_CLIENT_ADDR="127.0.0.1:9420" \
      TVSHOWS_GRPC_CLIENT_ADDR="127.0.0.1:9440" \
      MOVIES_HTTP_URL="http://127.0.0.1:9430" \
      TVSHOWS_HTTP_URL="http://127.0.0.1:9450" \
      REQUEST_MEDIA_HTTP_URL="http://127.0.0.1:9380" \
      "$BIN/mediauiprox" \
        -listen "${MEDIA_UI_LISTEN:-:5173}" \
        -dist "$UI_DIST" \
        -request-http "http://127.0.0.1:9380" \
        -auth-http "${AUTH_HTTP_URL:-http://127.0.0.1:9401}"
  else
    echo "WARN: mediauiprox missing — skipping media-ui" >&2
  fi
fi

if [[ -f "$ROOT/lib/common.sh" ]]; then
  # shellcheck disable=SC1091
  source "$ROOT/lib/common.sh"
  write_view_me "$ROOT" >/dev/null || true
fi

echo "started. logs in $RUN/"
echo "next: ./bootstrap-auth.sh && ./smoke-fixture.sh"
echo "URLs: $RUN/VIEW-ME.txt"
echo "stop:  ./up.sh stop"
