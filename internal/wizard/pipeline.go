package wizard

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Muxcore-Media/muxcore-installer/internal/assets"
	"github.com/Muxcore-Media/muxcore-installer/internal/compose"
	"github.com/Muxcore-Media/muxcore-installer/internal/envfile"
	"github.com/Muxcore-Media/muxcore-installer/internal/execstream"
	"github.com/Muxcore-Media/muxcore-installer/internal/fetch"
	"github.com/Muxcore-Media/muxcore-installer/internal/modules"
	"github.com/Muxcore-Media/muxcore-installer/internal/seedroots"
)

// StepID identifies one phase of the install pipeline.
type StepID string

const (
	StepConfigure StepID = "configure"
	StepFetch     StepID = "fetch"
	StepSeed      StepID = "seed"
	StepStart     StepID = "start"
	StepHealth    StepID = "health"
	StepAdmin     StepID = "admin"
	StepSmoke     StepID = "smoke"
)

// StepKind categorizes a Progress event within a step's lifecycle.
type StepKind int

const (
	KindStart StepKind = iota
	KindLine
	KindProgress
	KindDone
	KindWarn
	KindError
)

// Progress is one event the pipeline reports; the TUI renders it into the
// checklist/spinner (Start/Done/Error) and the viewport (Line/Warn/Error).
type Progress struct {
	Step  StepID
	Kind  StepKind
	Text  string
	Read  int64
	Total int64
}

// Emitter receives Progress events. Safe to call from the pipeline's goroutine.
type Emitter func(Progress)

// Steps is the fixed, ordered list shown in the work-phase checklist.
var Steps = []struct {
	ID    StepID
	Label string
}{
	{StepConfigure, "Write configuration"},
	{StepFetch, "Download MuxCore components"},
	{StepSeed, "Set up your library folders"},
	{StepStart, "Start MuxCore"},
	{StepHealth, "Wait for MuxCore to come online"},
	{StepAdmin, "Create your admin login"},
	{StepSmoke, "Run a health check"},
}

// Pipeline runs the actual install after the wizard has collected Answers.
type Pipeline struct {
	Answers *Answers
	Emit    Emitter
	DryRun  bool
}

func (p *Pipeline) emit(step StepID, kind StepKind, text string) {
	if p.Emit != nil {
		p.Emit(Progress{Step: step, Kind: kind, Text: text})
	}
}

// Run executes every step in order, stopping at the first hard failure.
func (p *Pipeline) Run(ctx context.Context) error {
	a := p.Answers
	a.EnabledModules = modules.Resolve(modules.Answers{
		Libraries: a.Libraries, Playback: a.Playback, Profile: a.Profile,
	})

	if err := p.stepConfigure(); err != nil {
		return err
	}
	if p.DryRun {
		p.emit(StepFetch, KindDone, "dry run — stopping before download and start")
		return nil
	}
	if err := p.stepFetch(ctx); err != nil {
		return err
	}
	if err := p.stepSeed(); err != nil {
		return err
	}
	if err := p.stepStart(ctx); err != nil {
		return err
	}
	if err := p.stepHealth(ctx); err != nil {
		return err
	}
	if err := p.stepAdmin(ctx); err != nil {
		p.emit(StepAdmin, KindWarn, "admin bootstrap failed (auth may still be starting): "+err.Error())
	}
	if err := p.stepSmoke(ctx); err != nil {
		p.emit(StepSmoke, KindWarn, "health check reported problems: "+err.Error())
	}
	return nil
}

func (p *Pipeline) stepConfigure() error {
	a := p.Answers
	p.emit(StepConfigure, KindStart, "")
	root := a.Root
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	if err := assets.ExtractInto(root); err != nil {
		return err
	}
	envPath := filepath.Join(root, ".env")
	examplePath := filepath.Join(root, ".env.example")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		if data, err := os.ReadFile(examplePath); err == nil {
			os.WriteFile(envPath, data, 0o600)
		}
	}
	f, err := envfile.Load(envPath)
	if err != nil {
		return err
	}

	kv := map[string]string{
		"INSTALL_RUNTIME":              a.Runtime,
		"MUXCORE_PROFILE":              a.Profile,
		"MUXCORE_LIBRARIES":            joinCSV(a.Libraries),
		"MUXCORE_PLAYBACK":             joinCSV(a.Playback),
		"MVP_ADMIN_USER":               a.AdminUser,
		"MVP_ADMIN_PASSWORD":           a.AdminPass,
		"MUXCORE_KEEP_MODE":            a.KeepMode,
		"MUXCORE_INSECURE_DISABLE_TLS": "true",
		"ENABLED_MODULES":              joinSpace(a.EnabledModules),
		"MVP_ENABLE_MEDIA_UI":          "0",
	}
	if a.HasPlayback("MuxCore player") {
		kv["MVP_ENABLE_MEDIA_UI"] = "1"
	}
	if p := a.LibraryPaths["movies"]; p != "" {
		kv["MVP_LIBRARY_ROOT"] = p
	}
	if p := a.LibraryPaths["tv"]; p != "" {
		kv["MVP_TV_LIBRARY_ROOT"] = p
	}
	if p := a.LibraryPaths["music"]; p != "" {
		kv["MVP_MUSIC_LIBRARY_ROOT"] = p
	}
	if p := a.LibraryPaths["books"]; p != "" {
		kv["MVP_BOOKS_LIBRARY_ROOT"] = p
	}
	if p := a.LibraryPaths["comics"]; p != "" {
		kv["MVP_COMICS_LIBRARY_ROOT"] = p
	}
	if p := a.LibraryPaths["audiobooks"]; p != "" {
		kv["MVP_AUDIOBOOKS_LIBRARY_ROOT"] = p
	}
	if a.WatchFolder && a.ImportDir != "" {
		kv["MVP_IMPORT_DIR"] = a.ImportDir
		kv["MVP_DOWNLOADS_DIR"] = a.ImportDir
	}
	if a.TMDBFixture {
		kv["TMDB_FIXTURE"] = "1"
		kv["TMDB_API_KEY"] = ""
	} else {
		kv["TMDB_FIXTURE"] = ""
		kv["TMDB_API_KEY"] = a.TMDBAPIKey
	}
	kv["MUXCORE_METADATA_LANGUAGE"] = a.MetadataLang
	if a.HasLibrary("Music") {
		kv["MUSICBRAINZ_FIXTURE"] = "1"
	}
	if a.Profile == "postgres" && a.DatabaseURL != "" {
		kv["DATABASE_URL"] = a.DatabaseURL
	}
	if a.Jellyfin.URL != "" {
		kv["JELLYFIN_BASE_URL"] = a.Jellyfin.URL
		kv["JELLYFIN_API_KEY"] = a.Jellyfin.Token
	}
	if a.Plex.URL != "" {
		kv["PLEX_URL"] = a.Plex.URL
		kv["PLEX_TOKEN"] = a.Plex.Token
	}
	if a.Emby.URL != "" {
		kv["EMBY_URL"] = a.Emby.URL
		kv["EMBY_TOKEN"] = a.Emby.Token
	}
	if a.HasPlayback("DLNA") {
		kv["DLNA_MEDIA_PATH"] = a.LibraryPaths["movies"]
	}
	if a.BindAllInterfaces {
		kv["MUXCORE_BIND_ALL"] = "1"
	}
	if a.HWAccelEnabled {
		kv["TRANSCODER_HWACCEL"] = a.HWAccelKind
	}
	f.SetAll(kv)
	if err := f.Save(); err != nil {
		return err
	}

	if a.Runtime == "compose" {
		_, err := compose.Write(compose.Env{
			Root:            root,
			EnabledModules:  a.EnabledModules,
			DatabaseURL:     a.DatabaseURL,
			TMDBAPIKey:      a.TMDBAPIKey,
			TMDBFixture:     kv["TMDB_FIXTURE"],
			MusicBrainzFix:  kv["MUSICBRAINZ_FIXTURE"],
			JellyfinURL:     a.Jellyfin.URL,
			JellyfinAPIKey:  a.Jellyfin.Token,
			PlexURL:         a.Plex.URL,
			PlexToken:       a.Plex.Token,
			EmbyURL:         a.Emby.URL,
			EmbyToken:       a.Emby.Token,
			MusicLibraryDir: a.LibraryPaths["music"],
			LibraryRoot:     a.LibraryPaths["movies"],
			TVLibraryRoot:   a.LibraryPaths["tv"],
			ImportDir:       a.ImportDir,
		})
		if err != nil {
			p.emit(StepConfigure, KindWarn, "compose file: "+err.Error())
		}
	}
	p.emit(StepConfigure, KindDone, "")
	return nil
}

func (p *Pipeline) stepFetch(ctx context.Context) error {
	a := p.Answers
	p.emit(StepFetch, KindStart, "")
	res, err := fetch.Run(fetch.Options{
		Root:       a.Root,
		Modules:    a.EnabledModules,
		LabBinDirs: fetch.LabBinDirs(a.Root),
		OnEvent: func(e fetch.Event) {
			switch e.Kind {
			case fetch.EventStart:
				p.emit(StepFetch, KindLine, fmt.Sprintf("- %s → %s", e.Module, e.Detail))
			case fetch.EventProgress:
				p.Emit(Progress{Step: StepFetch, Kind: KindProgress, Text: e.Module, Read: e.Read, Total: e.Total})
			case fetch.EventDetail:
				p.emit(StepFetch, KindLine, "    "+e.Detail)
			case fetch.EventSkipped:
				p.emit(StepFetch, KindLine, fmt.Sprintf("  %s already present", e.Module))
			case fetch.EventDone:
				p.emit(StepFetch, KindLine, fmt.Sprintf("  installed bin/%s", e.Module))
			case fetch.EventMissing:
				p.emit(StepFetch, KindWarn, fmt.Sprintf("  MISSING %s: %s", e.Module, e.Detail))
			}
		},
	})
	if err != nil {
		p.emit(StepFetch, KindError, err.Error())
		return err
	}
	if len(res.Missing) > 0 {
		p.emit(StepFetch, KindWarn, fmt.Sprintf("missing binaries: %v (place in bin/ or set MUXCORE_LAB_BIN)", res.Missing))
	}
	p.emit(StepFetch, KindDone, "")
	return nil
}

func (p *Pipeline) stepSeed() error {
	a := p.Answers
	p.emit(StepSeed, KindStart, "")
	db := filepath.Join(a.Root, "data", "roots", "roots.db")
	var roots []seedroots.Root
	add := func(kind, key, name string) {
		if pth := a.LibraryPaths[key]; pth != "" {
			roots = append(roots, seedroots.Root{Path: pth, Kind: kind, Name: name})
		}
	}
	add("movies", "movies", "Movies")
	add("tv", "tv", "TV")
	add("any", "music", "Music")
	add("any", "books", "Books")
	add("any", "comics", "Comics")
	add("any", "audiobooks", "Audiobooks")
	if err := seedroots.Seed(db, roots); err != nil {
		p.emit(StepSeed, KindWarn, "could not seed library folders: "+err.Error())
	}
	p.emit(StepSeed, KindDone, "")
	return nil
}

func (p *Pipeline) stepStart(ctx context.Context) error {
	a := p.Answers
	p.emit(StepStart, KindStart, "")
	sink := func(l execstream.Line) { p.emit(StepStart, lineKind(l), l.Text) }

	if a.Runtime == "compose" {
		bin, err := compose.Bin()
		if err != nil {
			p.emit(StepStart, KindError, err.Error())
			return err
		}
		args := append(bin[1:], "pull")
		if err := execstream.Command(ctx, "compose", a.Root, nil, sink, bin[0], args...); err != nil {
			p.emit(StepStart, KindWarn, "pull failed, trying anyway: "+err.Error())
		}
		upArgs := append(bin[1:], "up", "-d")
		if err := execstream.Command(ctx, "compose", a.Root, nil, sink, bin[0], upArgs...); err != nil {
			p.emit(StepStart, KindError, err.Error())
			return err
		}
		p.emit(StepStart, KindDone, "")
		return nil
	}

	switch a.KeepMode {
	case "user":
		if err := writeSystemdUnitFor(a.Root, false); err != nil {
			p.emit(StepStart, KindWarn, err.Error())
		} else {
			execstream.Command(ctx, "systemd", a.Root, nil, sink, "systemctl", "--user", "daemon-reload")
			if err := execstream.Command(ctx, "systemd", a.Root, nil, sink, "systemctl", "--user", "enable", "--now", "muxcore.service"); err == nil {
				p.emit(StepStart, KindDone, "")
				return nil
			}
			p.emit(StepStart, KindWarn, "user systemd unavailable — starting with up.sh instead")
		}
	case "system":
		if err := writeSystemdUnitFor(a.Root, true); err != nil {
			p.emit(StepStart, KindWarn, err.Error())
		} else {
			execstream.Command(ctx, "systemd", a.Root, nil, sink, "sudo", "systemctl", "daemon-reload")
			if err := execstream.Command(ctx, "systemd", a.Root, nil, sink, "sudo", "systemctl", "enable", "--now", "muxcore.service"); err == nil {
				p.emit(StepStart, KindDone, "")
				return nil
			}
			p.emit(StepStart, KindWarn, "system systemd unavailable — starting with up.sh instead")
		}
	}
	if err := execstream.Command(ctx, "up.sh", a.Root, nil, sink, filepath.Join(a.Root, "up.sh")); err != nil {
		p.emit(StepStart, KindError, err.Error())
		return err
	}
	p.emit(StepStart, KindDone, "")
	return nil
}

func (p *Pipeline) stepHealth(ctx context.Context) error {
	p.emit(StepHealth, KindStart, "")
	deadline := time.Now().Add(120 * time.Second)
	client := &http.Client{Timeout: 3 * time.Second}
	for {
		resp, err := client.Get("http://127.0.0.1:8080/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 || resp.StatusCode == 503 {
				p.emit(StepHealth, KindDone, "")
				return nil
			}
		}
		if time.Now().After(deadline) {
			err := fmt.Errorf("core /health did not become ready — check %s/run/core.log", p.Answers.Root)
			p.emit(StepHealth, KindError, err.Error())
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (p *Pipeline) stepAdmin(ctx context.Context) error {
	p.emit(StepAdmin, KindStart, "")
	sink := func(l execstream.Line) { p.emit(StepAdmin, lineKind(l), l.Text) }
	err := execstream.Command(ctx, "bootstrap-auth.sh", p.Answers.Root, nil, sink, filepath.Join(p.Answers.Root, "bootstrap-auth.sh"))
	if err != nil {
		return err
	}
	p.emit(StepAdmin, KindDone, "")
	return nil
}

func (p *Pipeline) stepSmoke(ctx context.Context) error {
	p.emit(StepSmoke, KindStart, "")
	sink := func(l execstream.Line) { p.emit(StepSmoke, lineKind(l), l.Text) }
	err := execstream.Command(ctx, "smoke-fixture.sh", p.Answers.Root, nil, sink, filepath.Join(p.Answers.Root, "smoke-fixture.sh"))
	if err != nil {
		return err
	}
	p.emit(StepSmoke, KindDone, "")
	return nil
}

func lineKind(l execstream.Line) StepKind {
	if l.Stderr {
		return KindWarn
	}
	return KindLine
}

// writeSystemdUnitFor mirrors write_systemd_user/write_systemd_system from
// onboard.sh. System units still need sudo to install; we only write the
// unit file here and let the caller drive systemctl (streamed to the viewport).
func writeSystemdUnitFor(root string, system bool) error {
	unitBody := fmt.Sprintf(`[Unit]
Description=MuxCore host stack
After=network-online.target

[Service]
Type=forking
WorkingDirectory=%s
ExecStart=%s/up.sh
ExecStop=%s/up.sh stop
Restart=on-failure

[Install]
WantedBy=%s
`, root, root, root, map[bool]string{true: "multi-user.target", false: "default.target"}[system])

	if !system {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dir := filepath.Join(home, ".config", "systemd", "user")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "muxcore.service"), []byte(unitBody), 0o644)
	}
	tmp, err := os.CreateTemp("", "muxcore-unit-*.service")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(unitBody); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	// System units require root; caller (sudo systemctl) does not copy the
	// file for us, so do it here with sudo install.
	cmd := exec.Command("sudo", "install", "-m", "0644", tmp.Name(), "/etc/systemd/system/muxcore.service")
	return cmd.Run()
}

func joinCSV(items []string) string {
	out := ""
	for i, v := range items {
		if i > 0 {
			out += ","
		}
		out += v
	}
	return out
}

func joinSpace(items []string) string {
	out := ""
	for i, v := range items {
		if i > 0 {
			out += " "
		}
		out += v
	}
	return out
}
