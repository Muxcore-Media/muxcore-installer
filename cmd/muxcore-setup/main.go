// Command muxcore-setup is MuxCore's installer: an interactive Bubbletea
// wizard for first-run setup, or a non-interactive pipeline driven by env
// vars (MUXCORE_NONINTERACTIVE=1) for CI/scripted installs. It replaces the
// old onboard.sh + install.sh bash wizard.
package main

import (
	"fmt"
	"os"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muxcore-Media/muxcore-installer/internal/mesh"
	"github.com/Muxcore-Media/muxcore-installer/internal/pathutil"
	"github.com/Muxcore-Media/muxcore-installer/internal/prereqs"
	"github.com/Muxcore-Media/muxcore-installer/internal/tui"
	"github.com/Muxcore-Media/muxcore-installer/internal/wizard"
)

// version is set via -ldflags -X main.version=... by scripts/build-release.sh.
var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	var devFlag, householdFlag bool
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-h", "--help":
			printUsage()
			return 0
		case "--version":
			fmt.Printf("muxcore-setup %s (see PIN-MATRIX.md for the module pin matrix baked into this build)\n", version)
			return 0
		case "--dev":
			devFlag = true
		case "--household":
			householdFlag = true
		default:
			fmt.Fprintf(os.Stderr, "error: unknown argument %q (see --help)\n", arg)
			return 2
		}
	}
	requested, err := wizard.ParseRequestedProfile(devFlag, householdFlag, os.Getenv("MUXCORE_PROFILE"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	if requested == mesh.ProfileDev {
		fmt.Fprintln(os.Stderr, "WARNING: dev security profile requested — the MuxCore mesh will run in plaintext and trust "+
			"modules by the ID they claim. Development only; never for real data or a reachable network.")
	}

	if prereqs.IsRoot() {
		fmt.Fprintln(os.Stderr, "error: do not run muxcore-setup as root. Run it as your normal user; it will ask for sudo only if you choose a system service or need optional packages.")
		return 1
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		fmt.Fprintln(os.Stderr, "error: unsupported OS. MuxCore's installer supports Linux and macOS (use WSL2 on Windows).")
		return 1
	}

	if os.Getenv("MUXCORE_NONINTERACTIVE") == "1" {
		return runNonInteractive(requested)
	}

	if !isTTY() {
		fmt.Fprintln(os.Stderr, `error: this installer needs a terminal.

Safer two-step:
  curl --proto '=https' --tlsv1.2 -fsSL https://getmuxcore.zem.systems -o get-muxcore.sh
  bash get-muxcore.sh`)
		return 1
	}

	root := os.Getenv("MUXCORE_INSTALL_DIR")
	if root == "" {
		root = pathutil.InstallRecommended()
	}
	root = pathutil.Resolve(root)

	m := tui.New(root)
	m.SetRequestedProfile(requested)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func isTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func printUsage() {
	fmt.Println(`muxcore-setup — MuxCore's interactive installer

Usage:
  muxcore-setup              run the interactive wizard
  MUXCORE_NONINTERACTIVE=1 MUXCORE_I_AGREE=1 muxcore-setup
                             scripted install from env vars (see README.md)

Security profile (ADR-0016):
  (default)                  household: TLS on every mesh hop, core CA in mesh/ca,
                             one enrolled identity per module. An existing install
                             that runs dev stays dev until you opt in.
  --household                switch an existing dev install to household (stops it,
                             keeps data/settings/logins, re-installs pinned binaries)
  --dev                      insecure dev profile (plaintext mesh) — development only

Env vars (non-interactive mode):
  MUXCORE_I_AGREE=1          required — accepts the acceptable-use agreement
  MUXCORE_DRY_RUN=1          stop before downloading or starting anything
  MUXCORE_INSTALL_DIR        install root (default: current directory)
  MUXCORE_LIBRARIES          CSV: Movies,TV,Music,Books,Comics,Audiobooks
  MUXCORE_PLAYBACK           CSV: MuxCore player,Jellyfin,Plex,Emby,DLNA
  INSTALL_RUNTIME            host | compose
  MUXCORE_KEEP_MODE          once | user | system | compose
  MUXCORE_BIND_ALL=1         listen on all interfaces (default: localhost only)
  MUXCORE_PROFILE            household | dev (same as --household / --dev)
  MUXCORE_DB_BACKEND         sqlite | postgres (default sqlite; legacy MUXCORE_PROFILE=sqlite|postgres still read, deprecated)
  DATABASE_URL               Postgres connection string (when profile=postgres)
  MVP_LIBRARY_ROOT           movie library path
  MVP_TV_LIBRARY_ROOT        TV library path
  MVP_MUSIC_LIBRARY_ROOT     music library path
  MVP_BOOKS_LIBRARY_ROOT     books library path
  MVP_COMICS_LIBRARY_ROOT    comics library path
  MVP_AUDIOBOOKS_LIBRARY_ROOT audiobooks library path
  MVP_IMPORT_DIR             watch/import folder
  MVP_ADMIN_USER             admin username (default: admin)
  MVP_ADMIN_PASSWORD         admin password (generated when unset)
  TMDB_API_KEY               live TMDB metadata (omit for offline fixture)
  MUXCORE_METADATA_LANGUAGE  e.g. en-US
  JELLYFIN_BASE_URL / JELLYFIN_API_KEY
  PLEX_URL / PLEX_TOKEN
  EMBY_URL / EMBY_TOKEN
  MUXCORE_REGISTRY           OCI image prefix for compose runtime`)
}
