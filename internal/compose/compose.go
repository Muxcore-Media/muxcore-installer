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

func image(name string) string {
	p := pins.Load()
	tag := p.ModuleTag(name)
	switch name {
	case "muxcored":
		return fmt.Sprintf("ghcr.io/muxcore-media/muxcored:%s", tag)
	case "mediauiprox":
		return fmt.Sprintf("ghcr.io/muxcore-media/media-ui:%s", tag)
	default:
		return fmt.Sprintf("ghcr.io/muxcore-media/%s:%s", name, tag)
	}
}

// Env is the subset of wizard answers the compose file needs.
type Env struct {
	Root            string
	EnabledModules  []string
	DatabaseURL     string
	TMDBAPIKey      string
	TMDBFixture     string
	MusicBrainzFix  string
	JellyfinURL     string
	JellyfinAPIKey  string
	PlexURL         string
	PlexToken       string
	EmbyURL         string
	EmbyToken       string
	DLNAMediaPath   string
	MusicLibraryDir string
	LibraryRoot     string
	TVLibraryRoot   string
	ImportDir       string
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
	name  string
	ports []string
	env   []string
	extra []string
}

func writeSvc(b *strings.Builder, root string, s svc) {
	fmt.Fprintf(b, "  %s:\n", s.name)
	fmt.Fprintf(b, "    image: %s\n", image(s.name))
	b.WriteString("    environment:\n")
	b.WriteString("      MUXCORE_GRPC_ADDR: core:9090\n")
	b.WriteString("      MUXCORE_INSECURE_DISABLE_TLS: \"true\"\n")
	fmt.Fprintf(b, "      MUXCORE_MODULE_ID: %s\n", s.name)
	b.WriteString("      MUXCORE_MESH_DIAL_LOCAL: \"true\"\n")
	for _, e := range s.env {
		fmt.Fprintf(b, "      %s\n", e)
	}
	b.WriteString("    volumes:\n")
	fmt.Fprintf(b, "      - %s/data:/data\n", root)
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

// Write renders docker-compose.yml for every enabled, non-core module plus
// the always-present core service.
func Write(env Env) (string, error) {
	var b strings.Builder
	b.WriteString("name: muxcore\nservices:\n  core:\n")
	fmt.Fprintf(&b, "    image: %s\n", image("muxcored"))
	b.WriteString(`    environment:
      MUXCORE_CONFIG: /app/muxcore.json
      MUXCORE_INSECURE_DISABLE_TLS: "true"
      MUXCORE_LOG_LEVEL: info
      MUXCORE_STORAGE_DIR: /app/data/storage
    ports:
      - "8080:8080"
      - "9090:9090"
    volumes:
`)
	fmt.Fprintf(&b, "      - %s/data:/app/data\n", env.Root)
	fmt.Fprintf(&b, "      - %s/muxcore.json:/app/muxcore.json:ro\n", env.Root)
	b.WriteString(`    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "curl -sf http://127.0.0.1:8080/health || exit 1"]
      interval: 5s
      timeout: 3s
      retries: 20
`)

	def := func(name string, ports []string, kv ...string) {
		if enabled(env, name) {
			writeSvc(&b, env.Root, svc{name: name, ports: ports, env: kv})
		}
	}

	def("api-rest", []string{"18080:8080"}, `API_REST_HTTP_ADDR: ":8080"`, `API_REST_GRPC_ADDR: ":9400"`)
	def("auth-local", []string{"9401:9401", "9403:9403"},
		`AUTH_DB_PATH: /data/auth/auth.db`, `AUTH_GRPC_ADDR: ":9403"`, `AUTH_HTTP_ADDR: ":9401"`)
	def("database-sqlite", nil, `SQLITE_DB_PATH: /data/sqlite/muxcore.db`)
	def("database-postgres", nil, fmt.Sprintf("DATABASE_URL: %s", env.DatabaseURL))
	def("secrets-file", nil, `SECRETS_STORE: /data/secrets/store.json`, `SECRETS_KEY_FILE: /data/secrets/master.key`)
	def("encryption-aesgcm", nil, `ENCRYPTION_KEY_FILE: /data/encryption/master.key`)
	def("call-policy-default", nil, `CALL_POLICY_FILE: /app/policies/call-policy.yaml`)
	def("publish-policy-default", nil, `PUBLISH_POLICY_FILE: /app/policies/publish-policy.yaml`)
	def("cache-local", nil, `CACHE_LOCAL_GRPC_ADDR: ":9600"`)
	def("ratelimit-tokenbucket", nil, `RATELIMIT_ENABLED: "false"`)
	def("health-monitor", []string{"9203:9203"}, `HEALTH_MONITOR_GRPC_ADDR: ":9202"`, `HEALTH_MONITOR_HTTP_ADDR: ":9203"`)
	def("admin-ui", []string{"8082:8082"}, `ADMIN_UI_ADDR: ":8082"`, `ADMIN_UI_CORE_ADDR: core:9090`,
		`ADMIN_UI_INSECURE: "true"`, `ADMIN_UI_AUTH_ADDR: http://auth-local:9401`)
	def("notification-default", nil, `NOTIFY_GRPC_ADDR: ":9441"`)
	def("metadata-tmdb", nil, fmt.Sprintf("TMDB_API_KEY: %s", env.TMDBAPIKey), fmt.Sprintf("TMDB_FIXTURE: %s", env.TMDBFixture))
	def("metadata-musicbrainz", nil, fmt.Sprintf("MUSICBRAINZ_FIXTURE: %s", env.MusicBrainzFix))
	def("media-movies", []string{"9430:9430"}, `MOVIES_DB_PATH: /data/movies/movies.db`, `MOVIES_IMAGE_DIR: /data/movies/images`, `MOVIES_HTTP_ADDR: ":9430"`)
	def("media-tvshows", nil, `TVSHOWS_DB_PATH: /data/tvshows/tvshows.db`, `TVSHOWS_IMAGE_DIR: /data/tvshows/images`, `TVSHOWS_GRPC_ADDR: ":9440"`, `TVSHOWS_HTTP_ADDR: ":9450"`)
	def("media-music", nil, `MUSIC_DATA_DIR: /data/music`, fmt.Sprintf("MUSIC_LIBRARY_DIR: %s", orDefault(env.MusicLibraryDir, "/data/library/music")))
	def("media-books", nil, `BOOKS_DATA_DIR: /data/books`)
	def("media-comics", nil, `COMICS_DATA_DIR: /data/comics`)
	def("media-audiobooks", nil, `AUDIOBOOKS_DATA_DIR: /data/audiobooks`)
	def("media-scanner", nil,
		`SCANNER_DB_PATH: /data/scanner/scanner.db`,
		fmt.Sprintf("SCANNER_LIBRARY_ROOT: %s", orDefault(env.LibraryRoot, "/data/library")),
		fmt.Sprintf("SCANNER_TV_LIBRARY_ROOT: %s", orDefault(env.TVLibraryRoot, "/data/library/tv")),
		fmt.Sprintf("SCANNER_DEFAULT_WATCH_DIR: %s", orDefault(env.ImportDir, "/data/import")),
		`SCANNER_IMPORT_MODE: copy`)
	def("media-root-folders", nil, `ROOTS_DB_PATH: /data/roots/roots.db`)
	def("media-rename", nil, `RENAME_DB_PATH: /data/rename/rename.db`)
	def("media-ffprobe", nil, `FFPROBE_DB_PATH: /data/ffprobe/cache.db`)
	def("media-subtitles", nil, `SUBS_DB_PATH: /data/subtitles/subtitles.db`, `SUBS_DIR: /data/subtitles/files`)
	def("media-custom-formats", nil, `FORMATS_DB_PATH: /data/formats/formats.db`, `FORMATS_SEED_DEFAULTS: "true"`)
	def("media-transcoder", nil, `TRANSCODER_GRPC_ADDR: ":9525"`, `TRANSCODER_HTTP_ADDR: ":9526"`)
	def("jellyfin", nil, fmt.Sprintf("JELLYFIN_BASE_URL: %s", env.JellyfinURL), fmt.Sprintf("JELLYFIN_API_KEY: %s", env.JellyfinAPIKey))
	def("plex", nil, fmt.Sprintf("PLEX_URL: %s", env.PlexURL), fmt.Sprintf("PLEX_TOKEN: %s", env.PlexToken))
	def("emby", nil, fmt.Sprintf("EMBY_URL: %s", env.EmbyURL), fmt.Sprintf("EMBY_TOKEN: %s", env.EmbyToken))
	def("media-dlna", nil, fmt.Sprintf("DLNA_MEDIA_PATH: %s", orDefault(env.DLNAMediaPath, orDefault(env.LibraryRoot, "/data/library"))))

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
