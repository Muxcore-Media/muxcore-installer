// Package compose generates a docker-compose.yml for the modules the wizard
// selected, for users who chose the Compose runtime instead of host
// processes. Ported from lib/compose.sh.
package compose

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Muxcore-Media/muxcore-installer/internal/mesh"
	"github.com/Muxcore-Media/muxcore-installer/internal/pins"
)

// Bin returns the compose command to shell out to (docker compose, podman
// compose, or a standalone docker-compose/podman-compose binary).
func Bin() ([]string, error) {
	if _, err := exec.LookPath("docker"); err == nil {
		if err := exec.Command("docker", "compose", "version").Run(); err == nil {
			return []string{"docker", "compose"}, nil
		}
	}
	if _, err := exec.LookPath("podman"); err == nil {
		if err := exec.Command("podman", "compose", "version").Run(); err == nil {
			return []string{"podman", "compose"}, nil
		}
	}
	if _, err := exec.LookPath("docker-compose"); err == nil {
		return []string{"docker-compose"}, nil
	}
	if _, err := exec.LookPath("podman-compose"); err == nil {
		return []string{"podman-compose"}, nil
	}
	return nil, fmt.Errorf("docker compose or podman compose is required for the Compose runtime")
}

// RegistryPrefix returns the OCI image prefix (LAN registry by default).
func RegistryPrefix() string {
	if v := os.Getenv("MUXCORE_REGISTRY"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "localhost:5000/muxcore"
}

func image(name string) string {
	p := pins.Load()
	tag := p.ModuleTag(name)
	prefix := RegistryPrefix()
	switch name {
	case "muxcored":
		return fmt.Sprintf("%s/muxcored:%s", prefix, tag)
	case "mediauiprox":
		return fmt.Sprintf("%s/media-ui:%s", prefix, p.ModuleTag("media-ui"))
	default:
		return fmt.Sprintf("%s/%s:%s", prefix, name, tag)
	}
}

// ModuleID is the mesh module ID (certificate CN) of a compose service.
// The MuxCore player BFF ships as mediauiprox but enrolls as media-ui.
func ModuleID(service string) string {
	if service == "mediauiprox" {
		return "media-ui"
	}
	return service
}

// ModuleIDs returns the mesh module IDs of every enabled module service
// (core excluded), plus media-ui when the player is enabled.
func ModuleIDs(enabledModules []string, enableMediaUI bool) []string {
	var out []string
	seen := map[string]bool{}
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, m := range enabledModules {
		if m == "muxcored" {
			continue
		}
		add(ModuleID(m))
	}
	if enableMediaUI {
		add("media-ui")
	}
	return out
}

// Env is the subset of wizard answers the compose file needs.
type Env struct {
	Root string
	// Profile is the security profile: household (default when empty) or dev.
	Profile               string
	EnabledModules        []string
	EnableMediaUI         bool
	MediaUIDist           string
	DatabaseURL           string
	TMDBAPIKey            string
	TMDBFixture           string
	MusicBrainzFix        string
	JellyfinURL           string
	JellyfinAPIKey        string
	PlexURL               string
	PlexToken             string
	EmbyURL               string
	EmbyToken             string
	DLNAMediaPath         string
	MusicLibraryDir       string
	LibraryRoot           string
	TVLibraryRoot         string
	BooksLibraryRoot      string
	ComicsLibraryRoot     string
	AudiobooksLibraryRoot string
	ImportDir             string
}

func enabled(env Env, name string) bool {
	for _, m := range env.EnabledModules {
		if m == name {
			return true
		}
	}
	return false
}

type svc struct {
	name    string
	ports   []string
	env     []string
	volumes []string
}

func household(env Env) bool { return env.Profile != mesh.ProfileDev }

// writeSvc renders one module service. In the household profile every
// module gets its own identity dir (bind mount of <root>/mesh/id/<id> at
// /mesh/id, MUXCORE_TLS_DIR), core's public CA read-only at /mesh/ca, and its
// single-use enrollment token from .env (ADR-0017). In dev it runs with the
// insecure flag, as before.
func writeSvc(b *strings.Builder, env Env, s svc) {
	root := env.Root
	id := ModuleID(s.name)
	fmt.Fprintf(b, "  %s:\n", s.name)
	fmt.Fprintf(b, "    image: %s\n", image(s.name))
	b.WriteString("    environment:\n")
	b.WriteString("      MUXCORE_GRPC_ADDR: core:9090\n")
	if household(env) {
		b.WriteString("      MUXCORE_PROFILE: household\n")
		b.WriteString("      MUXCORE_TLS_DIR: /mesh/id\n")
		b.WriteString("      MUXCORE_TLS_CA: /mesh/ca/ca.crt\n")
		fmt.Fprintf(b, "      MUXCORE_BOOTSTRAP_TOKEN: ${%s:?re-run muxcore-setup (mesh enrollment tokens in .env)}\n", mesh.TokenVar(id))
		// Peers dial a module by its compose service name, so that name
		// must be a SAN of its certificate (the module ID always is).
		if id == s.name {
			fmt.Fprintf(b, "      MUXCORE_ENROLL_DNS_NAMES: %s\n", s.name)
		}
	} else {
		b.WriteString("      MUXCORE_PROFILE: dev\n")
		b.WriteString("      MUXCORE_INSECURE_DISABLE_TLS: \"true\"\n")
	}
	fmt.Fprintf(b, "      MUXCORE_MODULE_ID: %s\n", id)
	b.WriteString("      MUXCORE_MESH_DIAL_LOCAL: \"true\"\n")
	for _, e := range s.env {
		fmt.Fprintf(b, "      %s\n", e)
	}
	b.WriteString("    volumes:\n")
	fmt.Fprintf(b, "      - %s/data:/data\n", root)
	if household(env) {
		fmt.Fprintf(b, "      - %s:/mesh/id\n", mesh.IdentityDir(root, id))
		fmt.Fprintf(b, "      - %s:/mesh/ca:ro\n", mesh.CAExportDir(root))
	}
	for _, v := range s.volumes {
		fmt.Fprintf(b, "      - %s\n", v)
	}
	if s.name == "call-policy-default" || s.name == "publish-policy-default" {
		fmt.Fprintf(b, "      - %s/policies:/app/policies:ro\n", root)
	}
	if len(s.ports) > 0 {
		b.WriteString("    ports:\n")
		for _, p := range s.ports {
			fmt.Fprintf(b, "      - %q\n", p)
		}
	}
	b.WriteString("    depends_on:\n      core:\n        condition: service_healthy\n")
	b.WriteString("    restart: unless-stopped\n")
}

func mountHostPath(hostPath, containerPath string) string {
	if hostPath == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s", hostPath, containerPath)
}

func libraryMounts(env Env) []string {
	var out []string
	add := func(host, container string) {
		if m := mountHostPath(host, container); m != "" {
			out = append(out, m)
		}
	}
	add(env.LibraryRoot, "/data/library")
	add(env.TVLibraryRoot, "/data/library/tv")
	add(env.MusicLibraryDir, "/data/library/music")
	add(env.BooksLibraryRoot, "/data/library/books")
	add(env.ComicsLibraryRoot, "/data/library/comics")
	add(env.AudiobooksLibraryRoot, "/data/library/audiobooks")
	add(env.ImportDir, "/data/import")
	return out
}

// Write renders docker-compose.yml for every enabled, non-core module plus
// the always-present core service.
func Write(env Env) (string, error) {
	var b strings.Builder
	b.WriteString("name: muxcore\nservices:\n  core:\n")
	fmt.Fprintf(&b, "    image: %s\n", image("muxcored"))
	b.WriteString(`    environment:
      MUXCORE_CONFIG: /app/muxcore.json
      MUXCORE_LOG_LEVEL: info
      MUXCORE_STORAGE_DIR: /app/data/storage
`)
	healthTest := `curl -sf http://127.0.0.1:8080/health || exit 1`
	if household(env) {
		// household (ADR-0016): TLS on gRPC and HTTP with core's own CA; the
		// CA key lives in <root>/mesh/ca, outside the data dir every module
		// mounts, and is never in a backup (ADR-0023).
		b.WriteString(`      MUXCORE_PROFILE: household
      MUXCORE_GRPC_CA_CERT_DIR: /app/ca
      MUXCORE_CA_EXPORT_DIR: /app/mesh-ca
      MUXCORE_TLS_SERVER_SANS: core
      MUXCORE_ENROLL_SECRET: ${MUXCORE_ENROLL_SECRET:?re-run muxcore-setup (mesh enrollment secret in .env)}
`)
		healthTest = `curl -sf --cacert /app/mesh-ca/ca.crt https://127.0.0.1:8080/health || exit 1`
	} else {
		b.WriteString(`      MUXCORE_PROFILE: dev
      MUXCORE_INSECURE_DISABLE_TLS: "true"
`)
	}
	b.WriteString(`    ports:
      - "8080:8080"
      - "9090:9090"
    volumes:
`)
	fmt.Fprintf(&b, "      - %s/data:/app/data\n", env.Root)
	if household(env) {
		fmt.Fprintf(&b, "      - %s:/app/ca\n", mesh.CADir(env.Root))
		fmt.Fprintf(&b, "      - %s:/app/mesh-ca\n", mesh.CAExportDir(env.Root))
	}
	fmt.Fprintf(&b, "      - %s/muxcore.json:/app/muxcore.json:ro\n", env.Root)
	for _, m := range libraryMounts(env) {
		fmt.Fprintf(&b, "      - %s\n", m)
	}
	fmt.Fprintf(&b, `    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", %q]
      interval: 5s`, healthTest)
	b.WriteString(`
      timeout: 3s
      retries: 20
`)

	libVol := libraryMounts(env)

	def := func(name string, ports []string, extraVol []string, kv ...string) {
		if enabled(env, name) {
			vols := append([]string{}, libVol...)
			vols = append(vols, extraVol...)
			writeSvc(&b, env, svc{name: name, ports: ports, env: kv, volumes: vols})
		}
	}

	def("api-rest", []string{"18080:8080"}, nil, `API_REST_HTTP_ADDR: ":8080"`, `API_REST_GRPC_ADDR: ":9400"`)
	// auth-local creates the admin (with the admin role) on an empty
	// database; bootstrap-auth.sh then only logs in.
	def("auth-local", []string{"9401:9401", "9403:9403"}, nil,
		`AUTH_DB_PATH: /data/auth/auth.db`, `AUTH_GRPC_ADDR: ":9403"`, `AUTH_HTTP_ADDR: ":9401"`,
		`AUTH_BOOTSTRAP_USER: ${MVP_ADMIN_USER:-admin}`, `AUTH_BOOTSTRAP_PASSWORD: ${MVP_ADMIN_PASSWORD:-}`)
	def("database-sqlite", nil, nil, `SQLITE_DB_PATH: /data/sqlite/muxcore.db`)
	def("database-postgres", nil, nil, fmt.Sprintf("DATABASE_URL: %s", env.DatabaseURL))
	def("secrets-file", nil, nil, `SECRETS_STORE: /data/secrets/store.json`, `SECRETS_KEY_FILE: /data/secrets/master.key`)
	def("encryption-aesgcm", nil, nil, `ENCRYPTION_KEY_FILE: /data/encryption/master.key`)
	def("call-policy-default", nil, nil, `CALL_POLICY_FILE: /app/policies/call-policy.yaml`)
	def("publish-policy-default", nil, nil, `PUBLISH_POLICY_FILE: /app/policies/publish-policy.yaml`)
	def("cache-local", nil, nil, `CACHE_LOCAL_GRPC_ADDR: ":9600"`)
	def("ratelimit-tokenbucket", nil, nil, `RATELIMIT_ENABLED: "false"`)
	def("health-monitor", []string{"9203:9203"}, nil, `HEALTH_MONITOR_GRPC_ADDR: ":9202"`, `HEALTH_MONITOR_HTTP_ADDR: ":9203"`)
	adminEnv := []string{`ADMIN_UI_ADDR: ":8082"`, `ADMIN_UI_CORE_ADDR: core:9090`, `ADMIN_UI_AUTH_ADDR: http://auth-local:9401`}
	if !household(env) {
		// dev only: ADMIN_UI_INSECURE also forces admin-ui's mesh dials to
		// plaintext, which household refuses.
		adminEnv = append(adminEnv, `ADMIN_UI_INSECURE: "true"`)
	}
	def("admin-ui", []string{"8082:8082"}, nil, adminEnv...)
	def("notification-default", nil, nil, `NOTIFY_GRPC_ADDR: ":9441"`)
	def("metadata-tmdb", nil, nil, fmt.Sprintf("TMDB_API_KEY: %s", env.TMDBAPIKey), fmt.Sprintf("TMDB_FIXTURE: %s", env.TMDBFixture))
	def("metadata-musicbrainz", nil, nil, fmt.Sprintf("MUSICBRAINZ_FIXTURE: %s", env.MusicBrainzFix))
	def("media-movies", []string{"9430:9430"}, nil, `MOVIES_DB_PATH: /data/movies/movies.db`, `MOVIES_IMAGE_DIR: /data/movies/images`, `MOVIES_HTTP_ADDR: ":9430"`)
	def("media-tvshows", nil, nil, `TVSHOWS_DB_PATH: /data/tvshows/tvshows.db`, `TVSHOWS_IMAGE_DIR: /data/tvshows/images`, `TVSHOWS_GRPC_ADDR: ":9440"`, `TVSHOWS_HTTP_ADDR: ":9450"`)
	def("media-music", nil, nil, `MUSIC_DATA_DIR: /data/music`, fmt.Sprintf("MUSIC_LIBRARY_DIR: %s", orDefault(env.MusicLibraryDir, "/data/library/music")))
	def("media-books", nil, nil, `BOOKS_DATA_DIR: /data/books`)
	def("media-comics", nil, nil, `COMICS_DATA_DIR: /data/comics`)
	def("media-audiobooks", nil, nil, `AUDIOBOOKS_DATA_DIR: /data/audiobooks`)
	def("media-scanner", nil, nil,
		`SCANNER_DB_PATH: /data/scanner/scanner.db`,
		fmt.Sprintf("SCANNER_LIBRARY_ROOT: %s", orDefault(env.LibraryRoot, "/data/library")),
		fmt.Sprintf("SCANNER_TV_LIBRARY_ROOT: %s", orDefault(env.TVLibraryRoot, "/data/library/tv")),
		fmt.Sprintf("SCANNER_DEFAULT_WATCH_DIR: %s", orDefault(env.ImportDir, "/data/import")),
		`SCANNER_IMPORT_MODE: copy`)
	def("media-root-folders", nil, nil, `ROOTS_DB_PATH: /data/roots/roots.db`)
	def("media-rename", nil, nil, `RENAME_DB_PATH: /data/rename/rename.db`)
	def("media-ffprobe", nil, nil, `FFPROBE_DB_PATH: /data/ffprobe/cache.db`)
	def("media-subtitles", nil, nil, `SUBS_DB_PATH: /data/subtitles/subtitles.db`, `SUBS_DIR: /data/subtitles/files`)
	def("media-custom-formats", nil, nil, `FORMATS_DB_PATH: /data/formats/formats.db`, `FORMATS_SEED_DEFAULTS: "true"`)
	def("media-transcoder", nil, nil, `TRANSCODER_GRPC_ADDR: ":9525"`, `TRANSCODER_HTTP_ADDR: ":9526"`)
	def("jellyfin", nil, nil, fmt.Sprintf("JELLYFIN_BASE_URL: %s", env.JellyfinURL), fmt.Sprintf("JELLYFIN_API_KEY: %s", env.JellyfinAPIKey))
	def("plex", nil, nil, fmt.Sprintf("PLEX_URL: %s", env.PlexURL), fmt.Sprintf("PLEX_TOKEN: %s", env.PlexToken))
	def("emby", nil, nil, fmt.Sprintf("EMBY_URL: %s", env.EmbyURL), fmt.Sprintf("EMBY_TOKEN: %s", env.EmbyToken))
	def("media-dlna", nil, nil, fmt.Sprintf("DLNA_MEDIA_PATH: %s", orDefault(env.DLNAMediaPath, orDefault(env.LibraryRoot, "/data/library"))))

	if env.EnableMediaUI {
		dist := env.MediaUIDist
		if dist == "" {
			dist = filepath.Join(env.Root, "dist-app")
		}
		var mediaVol []string
		if m := mountHostPath(dist, "/dist-app"); m != "" {
			mediaVol = append(mediaVol, m)
		}
		writeSvc(&b, env, svc{
			name:  "mediauiprox",
			ports: []string{"5173:5173"},
			env: []string{
				`MEDIA_UI_LISTEN: ":5173"`,
				`MEDIA_UI_DIST: /dist-app`,
				`MEDIA_UI_REQUIRE_AUTH: "1"`,
				`AUTH_HTTP_URL: http://auth-local:9401`,
			},
			volumes: append(libVol, mediaVol...),
		})
	}

	out := filepath.Join(env.Root, "docker-compose.yml")
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return out, nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// LoginGHCR logs the compose tool into ghcr.io using a GitHub token, best-effort.
func LoginGHCR(bin []string, token string) error {
	tool := "docker"
	if len(bin) > 0 && bin[0] == "podman" {
		tool = "podman"
	}
	cmd := exec.Command(tool, "login", "ghcr.io", "-u", "TOKEN", "--password-stdin")
	cmd.Stdin = strings.NewReader(token)
	return cmd.Run()
}
