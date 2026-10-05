package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Muxcore-Media/muxcore-installer/internal/pathutil"
	"github.com/Muxcore-Media/muxcore-installer/internal/wizard"
)

// runNonInteractive drives the exact same Pipeline the TUI uses, from env
// vars only, for CI and scripted installs (MUXCORE_NONINTERACTIVE=1). Output
// is plain, prefixed lines — no TUI dependency, safe for dumb terminals.
func runNonInteractive(requestedProfile string) int {
	if os.Getenv("MUXCORE_I_AGREE") != "1" {
		fmt.Fprintln(os.Stderr, "error: non-interactive install requires MUXCORE_I_AGREE=1")
		fmt.Fprintln(os.Stderr, "(MUXCORE_NONINTERACTIVE alone is not consent to the acceptable-use agreement)")
		return 1
	}

	root := os.Getenv("MUXCORE_INSTALL_DIR")
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		} else {
			root = pathutil.InstallDefault()
		}
	}
	root = pathutil.Resolve(root)

	a := wizard.Default(root)
	a.Agreed = true
	a.RequestedProfile = requestedProfile
	if v := os.Getenv("MUXCORE_LIBRARIES"); v != "" {
		a.Libraries = splitCSV(v)
	}
	if v := os.Getenv("MUXCORE_PLAYBACK"); v != "" {
		a.Playback = splitCSV(v)
	}
	if v := os.Getenv("INSTALL_RUNTIME"); v != "" {
		a.Runtime = v
	}
	if v := os.Getenv("MUXCORE_KEEP_MODE"); v != "" {
		a.KeepMode = v
	}
	be, legacy := wizard.ResolveDBBackend(os.Getenv("MUXCORE_DB_BACKEND"), os.Getenv("MUXCORE_PROFILE"), a.DBBackend)
	if legacy {
		fmt.Fprintln(os.Stderr, wizard.LegacyProfileWarning)
		_ = os.Unsetenv("MUXCORE_PROFILE") // do not leak the DB selector to core
	}
	a.DBBackend = be
	if v := os.Getenv("DATABASE_URL"); v != "" {
		a.DatabaseURL = v
	}
	a.MediaRoot = pathutil.MediaDefault()
	a.LibraryPaths = map[string]string{}
	for _, k := range libKindsEnv {
		if p := os.Getenv(k.env); p != "" {
			a.LibraryPaths[k.key] = pathutil.Resolve(p)
		}
	}
	a.WatchFolder = true
	if v := os.Getenv("MVP_IMPORT_DIR"); v != "" {
		a.ImportDir = pathutil.Resolve(v)
	}
	if v := os.Getenv("MVP_ADMIN_USER"); v != "" {
		a.AdminUser = v
	}
	if v := os.Getenv("MVP_ADMIN_PASSWORD"); v != "" {
		a.AdminPass = v
	}
	if err := a.EnsureAdminPassword(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if os.Getenv("TMDB_API_KEY") != "" {
		a.TMDBFixture = false
		a.TMDBAPIKey = os.Getenv("TMDB_API_KEY")
	}
	if v := os.Getenv("MUXCORE_METADATA_LANGUAGE"); v != "" {
		a.MetadataLang = v
	}
	a.Jellyfin = wizard.PlaybackCred{URL: os.Getenv("JELLYFIN_BASE_URL"), Token: os.Getenv("JELLYFIN_API_KEY")}
	a.Plex = wizard.PlaybackCred{URL: os.Getenv("PLEX_URL"), Token: os.Getenv("PLEX_TOKEN")}
	a.Emby = wizard.PlaybackCred{URL: os.Getenv("EMBY_URL"), Token: os.Getenv("EMBY_TOKEN")}
	a.BindAllInterfaces = os.Getenv("MUXCORE_BIND_ALL") == "1"

	dryRun := os.Getenv("MUXCORE_DRY_RUN") == "1"
	fmt.Printf("==> installing MuxCore into %s\n", root)

	pipe := &wizard.Pipeline{Answers: &a, DryRun: dryRun, Emit: func(p wizard.Progress) {
		printProgress(p)
	}}
	if err := pipe.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if !dryRun {
		fmt.Println("PASS: non-interactive install finished")
	}
	return 0
}

var libKindsEnv = []struct{ env, key string }{
	{"MVP_LIBRARY_ROOT", "movies"},
	{"MVP_TV_LIBRARY_ROOT", "tv"},
	{"MVP_MUSIC_LIBRARY_ROOT", "music"},
	{"MVP_BOOKS_LIBRARY_ROOT", "books"},
	{"MVP_COMICS_LIBRARY_ROOT", "comics"},
	{"MVP_AUDIOBOOKS_LIBRARY_ROOT", "audiobooks"},
}

func splitCSV(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func printProgress(p wizard.Progress) {
	prefix := "==>"
	switch p.Kind {
	case wizard.KindStart:
		fmt.Printf("%s %s\n", prefix, p.Step)
		return
	case wizard.KindWarn:
		prefix = "WARN:"
	case wizard.KindError:
		prefix = "FAIL:"
	case wizard.KindDone:
		if p.Text == "" {
			return
		}
	case wizard.KindProgress:
		return
	}
	if p.Text == "" {
		return
	}
	fmt.Printf("%s %s\n", prefix, p.Text)
}
