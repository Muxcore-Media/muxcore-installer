# Seed media-root-folders SQLite from wizard paths (before the module starts).
# shellcheck shell=bash

seed_root_sql() {
  local db="$1" path="$2" kind="$3" name="$4"
  path="$(resolve_install_dir "$path")"
  [[ -n "$path" ]] || return 0
  mkdir -p "$path" "$(dirname "$db")"
  local now id
  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  id="rf_$(date +%s)_${kind}"
  if command -v python3 >/dev/null 2>&1; then
    python3 - "$db" "$id" "$path" "$name" "$kind" "$now" <<'PY'
import sqlite3, sys
db, id_, path, name, kind, now = sys.argv[1:]
con = sqlite3.connect(db)
con.execute("PRAGMA journal_mode=WAL")
con.execute("""
CREATE TABLE IF NOT EXISTS root_folders (
  id TEXT PRIMARY KEY,
  path TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  media_kind TEXT NOT NULL DEFAULT 'any',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
)
""")
try:
    con.execute("ALTER TABLE root_folders ADD COLUMN naming_template_id TEXT NOT NULL DEFAULT ''")
except sqlite3.OperationalError:
    pass
con.execute(
    "INSERT OR IGNORE INTO root_folders (id, path, name, media_kind, naming_template_id, created_at, updated_at) VALUES (?,?,?,?, '', ?, ?)",
    (id_, path, name, kind, now, now),
)
con.commit()
con.close()
PY
    return 0
  fi
  if command -v sqlite3 >/dev/null 2>&1; then
    sqlite3 "$db" <<SQL
PRAGMA journal_mode=WAL;
CREATE TABLE IF NOT EXISTS root_folders (
  id TEXT PRIMARY KEY,
  path TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  media_kind TEXT NOT NULL DEFAULT 'any',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
INSERT OR IGNORE INTO root_folders (id, path, name, media_kind, created_at, updated_at)
VALUES ('$id', '$path', '$name', '$kind', '$now', '$now');
SQL
    return 0
  fi
  echo "WARN: no python3 or sqlite3 — library folders will need to be added in Admin UI" >&2
  return 0
}

seed_wizard_roots() {
  local root="$1"
  local db="$root/data/roots/roots.db"
  [[ -n "${MVP_LIBRARY_ROOT:-}" ]] && seed_root_sql "$db" "$MVP_LIBRARY_ROOT" movies Movies
  [[ -n "${MVP_TV_LIBRARY_ROOT:-}" ]] && seed_root_sql "$db" "$MVP_TV_LIBRARY_ROOT" tv TV
  [[ -n "${MVP_MUSIC_LIBRARY_ROOT:-}" ]] && seed_root_sql "$db" "$MVP_MUSIC_LIBRARY_ROOT" any Music
  [[ -n "${MVP_BOOKS_LIBRARY_ROOT:-}" ]] && seed_root_sql "$db" "$MVP_BOOKS_LIBRARY_ROOT" any Books
  [[ -n "${MVP_COMICS_LIBRARY_ROOT:-}" ]] && seed_root_sql "$db" "$MVP_COMICS_LIBRARY_ROOT" any Comics
  [[ -n "${MVP_AUDIOBOOKS_LIBRARY_ROOT:-}" ]] && seed_root_sql "$db" "$MVP_AUDIOBOOKS_LIBRARY_ROOT" any Audiobooks
}
