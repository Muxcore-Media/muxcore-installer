package wizard

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPlaybackIncludesMuxCorePlayer(t *testing.T) {
	a := Default("/tmp/install")
	if !a.HasPlayback("MuxCore player") {
		t.Fatal("Default() should include MuxCore player when Movies/TV are selected")
	}
}

func TestLoadFromEnvFilePreservesRestartOnlyFields(t *testing.T) {
	root := t.TempDir()
	env := filepath.Join(root, ".env")
	content := `MUXCORE_LIBRARIES="Movies,TV"
MUXCORE_PLAYBACK="MuxCore player"
MVP_LIBRARY_ROOT="/media/movies"
MVP_TV_LIBRARY_ROOT="/media/tv"
MVP_ADMIN_USER="admin"
MVP_ADMIN_PASSWORD="secret-pass"
MVP_ENABLE_MEDIA_UI="1"
INSTALL_RUNTIME="host"
MUXCORE_KEEP_MODE="once"
`
	if err := os.WriteFile(env, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := LoadFromEnvFile(root)
	if err != nil {
		t.Fatal(err)
	}
	if a.LibraryPaths["movies"] != "/media/movies" {
		t.Errorf("movies path = %q", a.LibraryPaths["movies"])
	}
	if a.AdminPass != "secret-pass" {
		t.Errorf("admin pass = %q", a.AdminPass)
	}
	if !a.HasPlayback("MuxCore player") {
		t.Error("expected MuxCore player from env")
	}
}

func TestPipelineDryRunStopsBeforeFetch(t *testing.T) {
	root := t.TempDir()
	a := Default(root)
	a.Agreed = true
	a.Root = root
	p := &Pipeline{Answers: &a, DryRun: true}
	if err := p.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "up.sh")); os.IsNotExist(err) {
		t.Fatal("dry run should still extract embedded assets")
	}
}

func TestPipelineRestartOnlyPreservesEnv(t *testing.T) {
	root := t.TempDir()
	envPath := filepath.Join(root, ".env")
	orig := `MUXCORE_LIBRARIES="Movies"
MVP_LIBRARY_ROOT="/keep/me"
MVP_ADMIN_PASSWORD="keep-secret"
`
	if err := os.WriteFile(envPath, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}
	a := Default(root)
	a.ExistingChoice = "restart-only"
	a.Agreed = true
	p := &Pipeline{Answers: &a, DryRun: true}
	if err := p.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !containsAll(s, "keep/me", "keep-secret") {
		t.Fatalf("restart-only clobbered .env:\n%s", s)
	}
}

func TestEnsureAdminPasswordGenerates(t *testing.T) {
	a := Default("/tmp")
	a.AdminPass = ""
	if err := a.EnsureAdminPassword(); err != nil {
		t.Fatal(err)
	}
	if len(a.AdminPass) != 16 {
		t.Fatalf("generated password length = %d", len(a.AdminPass))
	}
	if a.AdminPass == "admin-dev-only" {
		t.Fatal("must not use admin-dev-only")
	}
}

func TestEnsureAdminPasswordRefusesDevDefault(t *testing.T) {
	a := Default("/tmp")
	a.AdminPass = "admin-dev-only"
	if err := a.EnsureAdminPassword(); err != nil {
		t.Fatal(err)
	}
	if a.AdminPass == "admin-dev-only" {
		t.Fatal("should replace admin-dev-only")
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
