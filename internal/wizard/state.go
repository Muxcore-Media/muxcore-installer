package wizard

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Muxcore-Media/muxcore-installer/internal/envfile"
	"github.com/Muxcore-Media/muxcore-installer/internal/password"
)

// PlaybackCred holds the URL/token pair for an existing media server bridge.
type PlaybackCred struct {
	URL   string
	Token string
	OK    bool // true once a live connection check has succeeded
}

// Answers is every decision the wizard makes, in collection order.
type Answers struct {
	// Legal
	Agreed bool

	// Install location + existing-install handling
	Root           string
	ExistingFound  bool
	ExistingChoice string // "reconfigure" | "restart-only" | "fresh"

	// How MuxCore runs
	Runtime  string // host | compose
	KeepMode string // once | user | system | compose

	// Libraries
	Libraries []string // Movies, TV, Music, Books, Comics, Audiobooks

	// Media layout
	MediaRoot    string
	LibraryPaths map[string]string // kind -> path (movies, tv, music, books, comics, audiobooks)
	WatchFolder  bool
	ImportDir    string

	// Playback
	Playback []string // "MuxCore player", Jellyfin, Plex, Emby, DLNA
	Jellyfin PlaybackCred
	Plex     PlaybackCred
	Emby     PlaybackCred

	// Network exposure
	BindAllInterfaces bool

	// Admin login
	AdminUser string
	AdminPass string

	// Metadata
	TMDBFixture   bool
	TMDBAPIKey    string
	TMDBValidated bool
	MetadataLang  string // e.g. en-US

	// Database
	Profile     string // sqlite | postgres
	DatabaseURL string

	// Hardware transcoding
	HWAccelEnabled bool
	HWAccelKind    string

	// Derived
	EnabledModules []string
}

// Default returns an Answers pre-populated the same way onboard.sh's
// defaults worked, so a bare `MUXCORE_NONINTERACTIVE=1` run behaves the same.
func Default(root string) Answers {
	return Answers{
		Root:         root,
		Runtime:      "host",
		KeepMode:     "once",
		Libraries:    []string{"Movies", "TV"},
		LibraryPaths: map[string]string{},
		WatchFolder:  true,
		Playback:     []string{"MuxCore player"},
		AdminUser:    "admin",
		TMDBFixture:  true,
		MetadataLang: "en-US",
		Profile:      "sqlite",
	}
}

// HasLibrary reports whether kind (case-sensitive, e.g. "Movies") is selected.
func (a *Answers) HasLibrary(kind string) bool {
	for _, k := range a.Libraries {
		if k == kind {
			return true
		}
	}
	return false
}

// HasPlayback reports whether a playback option is selected.
func (a *Answers) HasPlayback(kind string) bool {
	for _, k := range a.Playback {
		if k == kind {
			return true
		}
	}
	return false
}

// LoadFromEnvFile reads an existing install's .env into Answers for
// restart-only upgrades (preserve library paths, playback, admin creds, etc.).
func LoadFromEnvFile(root string) (*Answers, error) {
	envPath := filepath.Join(root, ".env")
	f, err := envfile.Load(envPath)
	if err != nil {
		return nil, err
	}
	a := Default(root)
	a.Root = root
	a.Runtime = f.Get("INSTALL_RUNTIME", a.Runtime)
	a.KeepMode = f.Get("MUXCORE_KEEP_MODE", a.KeepMode)
	a.Profile = f.Get("MUXCORE_PROFILE", a.Profile)
	a.DatabaseURL = f.Get("DATABASE_URL", "")
	a.AdminUser = f.Get("MVP_ADMIN_USER", a.AdminUser)
	a.AdminPass = f.Get("MVP_ADMIN_PASSWORD", "")
	a.MetadataLang = f.Get("MUXCORE_METADATA_LANGUAGE", a.MetadataLang)
	a.TMDBAPIKey = f.Get("TMDB_API_KEY", "")
	a.TMDBFixture = f.Get("TMDB_FIXTURE", "1") == "1"
	a.BindAllInterfaces = f.Get("MUXCORE_BIND_ALL", "") == "1"
	a.HWAccelKind = f.Get("TRANSCODER_HWACCEL", "")

	if libs := f.Get("MUXCORE_LIBRARIES", ""); libs != "" {
		a.Libraries = splitCSV(libs)
	}
	if pb := f.Get("MUXCORE_PLAYBACK", ""); pb != "" {
		a.Playback = splitCSV(pb)
	} else if f.Get("MVP_ENABLE_MEDIA_UI", "0") == "1" {
		a.Playback = []string{"MuxCore player"}
	}

	a.LibraryPaths = map[string]string{}
	for key, envKey := range map[string]string{
		"movies": "MVP_LIBRARY_ROOT", "tv": "MVP_TV_LIBRARY_ROOT",
		"music": "MVP_MUSIC_LIBRARY_ROOT", "books": "MVP_BOOKS_LIBRARY_ROOT",
		"comics": "MVP_COMICS_LIBRARY_ROOT", "audiobooks": "MVP_AUDIOBOOKS_LIBRARY_ROOT",
	} {
		if p := f.Get(envKey, ""); p != "" {
			a.LibraryPaths[key] = p
		}
	}
	if imp := f.Get("MVP_IMPORT_DIR", ""); imp != "" {
		a.ImportDir = imp
		a.WatchFolder = true
	}
	a.Jellyfin = PlaybackCred{URL: f.Get("JELLYFIN_BASE_URL", ""), Token: f.Get("JELLYFIN_API_KEY", "")}
	a.Plex = PlaybackCred{URL: f.Get("PLEX_URL", ""), Token: f.Get("PLEX_TOKEN", "")}
	a.Emby = PlaybackCred{URL: f.Get("EMBY_URL", ""), Token: f.Get("EMBY_TOKEN", "")}
	return &a, nil
}

// EnsureAdminPassword generates a password when unset (never admin-dev-only).
func (a *Answers) EnsureAdminPassword() error {
	if a.AdminPass != "" && a.AdminPass != "admin-dev-only" {
		return nil
	}
	if v := os.Getenv("MVP_ADMIN_PASSWORD"); v != "" && v != "admin-dev-only" {
		a.AdminPass = v
		return nil
	}
	a.AdminPass = password.Generate(16)
	return nil
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
