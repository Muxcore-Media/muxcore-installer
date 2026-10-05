package wizard

import (
	"os"

	"github.com/Muxcore-Media/muxcore-installer/internal/envfile"
)

// LegacyProfileWarning is printed when the deprecated MUXCORE_PROFILE=sqlite|postgres
// database selector is used. Core v0.6.10+ reserves MUXCORE_PROFILE for the
// security profile (dev|household).
const LegacyProfileWarning = "warning: MUXCORE_PROFILE=sqlite|postgres is deprecated for selecting the database; " +
	"use MUXCORE_DB_BACKEND (MUXCORE_PROFILE is reserved for the core security profile dev|household)"

func isDBBackend(v string) bool { return v == "sqlite" || v == "postgres" }

// ResolveDBBackend picks the database backend. MUXCORE_DB_BACKEND wins; if it
// is empty and the legacy MUXCORE_PROFILE holds sqlite|postgres, that value is
// used and legacy is true. Otherwise def is returned.
func ResolveDBBackend(backend, legacyProfile, def string) (value string, legacy bool) {
	if backend != "" {
		return backend, false
	}
	if isDBBackend(legacyProfile) {
		return legacyProfile, true
	}
	return def, false
}

// MigrateLegacyProfile rewrites MUXCORE_PROFILE=sqlite|postgres in f to
// MUXCORE_DB_BACKEND (an existing MUXCORE_DB_BACKEND wins). Other
// MUXCORE_PROFILE values (dev|household) are left untouched. It reports
// whether f was changed; the caller saves.
func MigrateLegacyProfile(f *envfile.File) bool {
	old := f.Get("MUXCORE_PROFILE", "")
	if !isDBBackend(old) {
		return false
	}
	if f.Get("MUXCORE_DB_BACKEND", "") == "" {
		f.Set("MUXCORE_DB_BACKEND", old)
	}
	f.Delete("MUXCORE_PROFILE")
	return true
}

// migrateEnvFile migrates an existing .env at path in place. Missing file is a no-op.
func migrateEnvFile(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		return false, nil
	}
	f, err := envfile.Load(path)
	if err != nil {
		return false, err
	}
	if !MigrateLegacyProfile(f) {
		return false, nil
	}
	return true, f.Save()
}
