// Package wizard holds the answer set the installer collects, independent
// of whether it was gathered by the interactive TUI or from env vars in
// non-interactive/CI mode. Both cmd/muxcore-setup/main.go paths build one of
// these and hand it to the same install pipeline.
package wizard

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
		Playback:     nil,
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
