package wizard

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Muxcore-Media/muxcore-installer/internal/assets"
	"github.com/Muxcore-Media/muxcore-installer/internal/compose"
	"github.com/Muxcore-Media/muxcore-installer/internal/envfile"
	"github.com/Muxcore-Media/muxcore-installer/internal/execstream"
	"github.com/Muxcore-Media/muxcore-installer/internal/fetch"
	"github.com/Muxcore-Media/muxcore-installer/internal/mesh"
	"github.com/Muxcore-Media/muxcore-installer/internal/modules"
	"github.com/Muxcore-Media/muxcore-installer/internal/prereqs"
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

	if a.ExistingChoice == "restart-only" {
		loaded, err := LoadFromEnvFile(a.Root)
		if err != nil {
			return fmt.Errorf("load existing .env: %w", err)
		}
		// Preserve wizard-only fields (legal consent, existing choice).
		loaded.Agreed = a.Agreed
		loaded.ExistingChoice = a.ExistingChoice
		loaded.ExistingFound = a.ExistingFound
		loaded.RequestedProfile = a.RequestedProfile
		p.Answers = loaded
		a = p.Answers
	}

	a.EnabledModules = modules.Resolve(modules.Answers{
		Libraries: a.Libraries, Playback: a.Playback, Profile: a.DBBackend,
	})

	if err := a.EnsureAdminPassword(); err != nil {
		return err
	}

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
	if a.ExistingChoice != "restart-only" {
		if err := p.stepSeed(); err != nil {
			return err
		}
	} else {
		p.emit(StepSeed, KindDone, "skipped — restart-only")
	}
	if err := p.stepStart(ctx); err != nil {
		return err
	}
	if err := p.stepHealth(ctx); err != nil {
		return err
	}
	if err := p.stepAdmin(ctx); err != nil {
		p.emit(StepAdmin, KindError, err.Error())
		return err
	}
	if err := p.stepSmoke(ctx); err != nil {
		p.emit(StepSmoke, KindError, err.Error())
		return err
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

	restartOnly := a.ExistingChoice == "restart-only"
	envPath := filepath.Join(root, ".env")
	if migrated, err := migrateEnvFile(envPath); err != nil {
		p.emit(StepConfigure, KindWarn, "could not migrate legacy MUXCORE_PROFILE in .env: "+err.Error())
	} else if migrated {
		p.emit(StepConfigure, KindLine, "migrated .env: MUXCORE_PROFILE=sqlite|postgres -> MUXCORE_DB_BACKEND (MUXCORE_PROFILE is now reserved for core's security profile)")
	}

	// Security profile (ADR-0016): decided before the assets are extracted,
	// because switching an existing dev install moves its old scripts aside.
	existing, err := loadExistingEnv(envPath)
	if err != nil {
		return err
	}
	plan, err := planSecurity(existing, a.ExistingChoice, a.RequestedProfile)
	if err != nil {
		return err
	}
	if plan.Migrate && p.DryRun {
		p.emit(StepConfigure, KindWarn, "dry run: would switch this install from the dev profile to household; leaving it on dev")
		plan.Profile, plan.Migrate = plan.Existing, false
	}
	kind, note := planNote(plan, a.RequestedProfile)
	p.emit(StepConfigure, kind, note)
	if plan.Migrate {
		if err := p.migrateToHousehold(root); err != nil {
			return err
		}
	}
	a.SecurityProfile = plan.Profile

	if err := assets.ExtractInto(root); err != nil {
		return err
	}
	examplePath := filepath.Join(root, ".env.example")

	if restartOnly {
		if existing == nil {
			return fmt.Errorf("restart-only requires existing .env at %s", envPath)
		}
		p.emit(StepConfigure, KindLine, "preserving existing .env (restart-only)")
		if plan.Profile != plan.Existing {
			// Only the security settings change; library paths, playback and
			// logins stay as they are.
			if err := applySecurityEnv(existing, plan.Profile, a.Runtime, a.moduleIDs()); err != nil {
				return err
			}
			if err := existing.Save(); err != nil {
				return err
			}
			p.emit(StepConfigure, KindLine, "updated the security settings in .env (MUXCORE_PROFILE="+plan.Profile+")")
		}
		if err := p.ensureMeshDirs(); err != nil {
			return err
		}
		if err := p.writeViewMe(root); err != nil {
			p.emit(StepConfigure, KindWarn, "VIEW-ME.txt: "+err.Error())
		}
		if a.Runtime == "compose" {
			if err := p.writeCompose(root); err != nil {
				p.emit(StepConfigure, KindWarn, "compose file: "+err.Error())
			}
		}
		p.emit(StepConfigure, KindDone, "")
		return nil
	}

	if existing == nil {
		if data, err := os.ReadFile(examplePath); err == nil {
			if werr := os.WriteFile(envPath, data, 0o600); werr != nil {
				p.emit(StepConfigure, KindWarn, "could not seed .env from the example: "+werr.Error())
			}
		}
	}
	f, err := envfile.Load(envPath)
	if err != nil {
		return err
	}

	kv := p.envKV(a)
	f.SetAll(kv)
	if err := applySecurityEnv(f, plan.Profile, a.Runtime, a.moduleIDs()); err != nil {
		return err
	}
	if err := f.Save(); err != nil {
		return err
	}
	if err := p.ensureMeshDirs(); err != nil {
		return err
	}
	if err := p.writeViewMe(root); err != nil {
		p.emit(StepConfigure, KindWarn, "VIEW-ME.txt: "+err.Error())
	}
	if a.Runtime == "compose" {
		if err := p.writeCompose(root); err != nil {
			p.emit(StepConfigure, KindWarn, "compose file: "+err.Error())
		}
	}
	p.emit(StepConfigure, KindDone, "")
	return nil
}

// ensureMeshDirs creates mesh/ (CA, public CA, one identity dir per module)
// in the household profile. up.sh does the same on every start; the compose
// runtime bind-mounts these directories, so they must exist first.
func (p *Pipeline) ensureMeshDirs() error {
	a := p.Answers
	if a.SecurityProfile != mesh.ProfileHousehold {
		return nil
	}
	return mesh.EnsureDirs(a.Root, a.moduleIDs())
}

func (p *Pipeline) envKV(a *Answers) map[string]string {
	kv := map[string]string{
		"INSTALL_RUNTIME":     a.Runtime,
		"MUXCORE_DB_BACKEND":  a.DBBackend,
		"MUXCORE_LIBRARIES":   joinCSV(a.Libraries),
		"MUXCORE_PLAYBACK":    joinCSV(a.Playback),
		"MVP_ADMIN_USER":      a.AdminUser,
		"MVP_ADMIN_PASSWORD":  a.AdminPass,
		"MUXCORE_KEEP_MODE":   a.KeepMode,
		"ENABLED_MODULES":     joinSpace(a.EnabledModules),
		"MVP_ENABLE_MEDIA_UI": "0",
	}
	if a.HasPlayback("MuxCore player") {
		kv["MVP_ENABLE_MEDIA_UI"] = "1"
		kv["MEDIA_UI_DIST"] = filepath.Join(a.Root, "dist-app")
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
	if a.DBBackend == "postgres" && a.DatabaseURL != "" {
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
	return kv
}

func (p *Pipeline) writeCompose(root string) error {
	a := p.Answers
	kv := p.envKV(a)
	_, err := compose.Write(compose.Env{
		Root:                  root,
		Profile:               a.SecurityProfile,
		EnabledModules:        a.EnabledModules,
		EnableMediaUI:         a.HasPlayback("MuxCore player"),
		MediaUIDist:           kv["MEDIA_UI_DIST"],
		DatabaseURL:           a.DatabaseURL,
		TMDBAPIKey:            a.TMDBAPIKey,
		TMDBFixture:           kv["TMDB_FIXTURE"],
		MusicBrainzFix:        kv["MUSICBRAINZ_FIXTURE"],
		JellyfinURL:           a.Jellyfin.URL,
		JellyfinAPIKey:        a.Jellyfin.Token,
		PlexURL:               a.Plex.URL,
		PlexToken:             a.Plex.Token,
		EmbyURL:               a.Emby.URL,
		EmbyToken:             a.Emby.Token,
		DLNAMediaPath:         a.LibraryPaths["movies"],
		MusicLibraryDir:       a.LibraryPaths["music"],
		LibraryRoot:           a.LibraryPaths["movies"],
		TVLibraryRoot:         a.LibraryPaths["tv"],
		BooksLibraryRoot:      a.LibraryPaths["books"],
		ComicsLibraryRoot:     a.LibraryPaths["comics"],
		AudiobooksLibraryRoot: a.LibraryPaths["audiobooks"],
		ImportDir:             a.ImportDir,
	})
	return err
}

func (p *Pipeline) writeViewMe(root string) error {
	a := p.Answers
	out := filepath.Join(root, "run", "VIEW-ME.txt")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("MuxCore — you're ready\n\n")
	fmt.Fprintf(&b, "  Admin UI:     http://localhost:8082\n                login: %s / %s\n", a.AdminUser, a.AdminPass)
	if a.HasPlayback("MuxCore player") {
		b.WriteString("  Player:       http://127.0.0.1:5173\n")
	}
	fmt.Fprintf(&b, "\n  Core health:  %s\n", coreHealthHint(a.SecurityProfile))
	b.WriteString("\n" + securitySummary(a.SecurityProfile) + "\n")
	b.WriteString("\nStart:  ./up.sh\nStop:   ./up.sh stop\n")
	return os.WriteFile(out, []byte(b.String()), 0o600)
}

func (p *Pipeline) stepFetch(ctx context.Context) error {
	a := p.Answers
	p.emit(StepFetch, KindStart, "")
	_, err := fetch.Run(fetch.Options{
		Root:            a.Root,
		Modules:         a.EnabledModules,
		LabBinDirs:      fetch.LabBinDirs(a.Root),
		FetchMediaUIApp: a.HasPlayback("MuxCore player"),
		OnEvent: func(e fetch.Event) {
			switch e.Kind {
			case fetch.EventStart:
				p.emit(StepFetch, KindLine, fmt.Sprintf("- %s → %s", e.Module, e.Detail))
				p.Emit(Progress{Step: StepFetch, Kind: KindProgress, Text: e.Module})
			case fetch.EventProgress:
				p.Emit(Progress{Step: StepFetch, Kind: KindProgress, Text: e.Module, Read: e.Read, Total: e.Total})
			case fetch.EventDetail:
				p.emit(StepFetch, KindLine, "    "+e.Detail)
			case fetch.EventSkipped:
				p.emit(StepFetch, KindLine, fmt.Sprintf("  %s already present", e.Module))
			case fetch.EventDone:
				p.emit(StepFetch, KindLine, fmt.Sprintf("  installed %s", e.Module))
			case fetch.EventMissing:
				p.emit(StepFetch, KindError, fmt.Sprintf("MISSING %s: %s", e.Module, e.Detail))
			}
		},
	})
	if err != nil {
		p.emit(StepFetch, KindError, err.Error())
		return err
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
		if !prereqs.HaveDocker() {
			msg := "the Docker daemon isn't reachable — start Docker (Docker Desktop, or `sudo systemctl start docker`) and try again"
			p.emit(StepStart, KindError, msg)
			return errors.New(msg)
		}
		args := append(bin[1:], "pull")
		if err := execstream.Command(ctx, "compose", a.Root, nil, sink, bin[0], args...); err != nil {
			p.emit(StepStart, KindWarn, "pull failed, trying anyway: "+err.Error())
		}
		upArgs := append(bin[1:], "up", "-d")
		if err := execstream.Command(ctx, "compose", a.Root, nil, sink, bin[0], upArgs...); err != nil {
			p.emit(StepStart, KindError, "docker compose up failed — is the Docker daemon still running? ("+err.Error()+")")
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
			_ = execstream.Command(ctx, "systemd", a.Root, nil, sink, "systemctl", "--user", "daemon-reload")
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
			_ = execstream.Command(ctx, "systemd", a.Root, nil, sink, "sudo", "systemctl", "daemon-reload")
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
	a := p.Answers
	deadline := time.Now().Add(120 * time.Second)
	url := "http://127.0.0.1:8080/health"
	if a.SecurityProfile == mesh.ProfileHousehold {
		// household: core's HTTP port is HTTPS with its auto-issued
		// certificate (SANs include 127.0.0.1); verify it against the CA core
		// exports to mesh/public once it is up.
		url = "https://127.0.0.1:8080/health"
	}
	for {
		client, err := healthClient(a)
		if err == nil {
			resp, gerr := client.Get(url)
			if gerr == nil {
				code := resp.StatusCode
				_ = resp.Body.Close()
				if code == http.StatusOK {
					p.emit(StepHealth, KindDone, "")
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			err := fmt.Errorf("core %s did not return HTTP 200 — check %s/run/core.log", url, a.Root)
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

// healthClient returns the HTTP client for the core health probe: plain in
// dev, TLS verified against core's exported CA in household (an error until
// core has exported it).
func healthClient(a *Answers) (*http.Client, error) {
	if a.SecurityProfile != mesh.ProfileHousehold {
		return &http.Client{Timeout: 3 * time.Second}, nil
	}
	pem, err := os.ReadFile(mesh.CACertPath(a.Root))
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no certificate in %s", mesh.CACertPath(a.Root))
	}
	return &http.Client{
		Timeout:   3 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
	}, nil
}

// coreHealthHint is the core health URL line for VIEW-ME.txt.
func coreHealthHint(profile string) string {
	if profile == mesh.ProfileHousehold {
		return "https://127.0.0.1:8080/health  (curl --cacert mesh/public/ca.crt …)"
	}
	return "http://127.0.0.1:8080/health"
}

// securitySummary is the security paragraph for VIEW-ME.txt.
func securitySummary(profile string) string {
	if profile == mesh.ProfileHousehold {
		return "Security:     household profile — TLS on every mesh hop, one enrolled identity per module.\n" +
			"              mesh/ holds the CA key and module keys: never back it up (ADR-0023).\n" +
			"              Lost a module identity? ./bin/muxcored enroll reset <id> (see README)."
	}
	return "Security:     DEV profile — plaintext mesh, development only. " + MigrateHint + "."
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
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.WriteString(unitBody); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
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
