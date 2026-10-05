// Package seedroots writes media-root-folders' SQLite rows for the paths the
// wizard collected, before that module starts for the first time. Ported
// from lib/seed-roots.sh, using a pure-Go SQLite driver so the installer
// binary needs no CGO/system sqlite3.
package seedroots

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Root describes one library root folder to seed.
type Root struct {
	Path string
	Kind string // movies | tv | any
	Name string
}

// Seed opens (creating if needed) db and inserts any roots not already present.
func Seed(db string, roots []Root) error {
	if len(roots) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(db), 0o755); err != nil {
		return err
	}
	conn, err := sql.Open("sqlite", db)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return err
	}
	if _, err := conn.Exec(`
CREATE TABLE IF NOT EXISTS root_folders (
  id TEXT PRIMARY KEY,
  path TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  media_kind TEXT NOT NULL DEFAULT 'any',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
)`); err != nil {
		return err
	}
	// Best-effort column add for older DBs, same as the bash version's ALTER TABLE.
	_, _ = conn.Exec(`ALTER TABLE root_folders ADD COLUMN naming_template_id TEXT NOT NULL DEFAULT ''`)

	now := time.Now().UTC().Format(time.RFC3339)
	for _, r := range roots {
		if r.Path == "" {
			continue
		}
		if err := os.MkdirAll(r.Path, 0o755); err != nil {
			return err
		}
		id := fmt.Sprintf("rf_%d_%s", time.Now().UnixNano(), r.Kind)
		_, err := conn.Exec(
			`INSERT OR IGNORE INTO root_folders (id, path, name, media_kind, naming_template_id, created_at, updated_at) VALUES (?,?,?,?,'',?,?)`,
			id, r.Path, r.Name, r.Kind, now, now,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
