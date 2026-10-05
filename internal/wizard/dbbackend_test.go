package wizard

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDBBackend(t *testing.T) {
	cases := []struct {
		backend, legacy, def, want string
		wantLegacy                 bool
	}{
		{"", "", "sqlite", "sqlite", false},
		{"postgres", "", "sqlite", "postgres", false},
		{"", "postgres", "sqlite", "postgres", true},
		{"", "sqlite", "sqlite", "sqlite", true},
		{"sqlite", "postgres", "sqlite", "sqlite", false}, // new var wins
		{"", "household", "sqlite", "sqlite", false},      // security profile, not a DB value
		{"", "dev", "sqlite", "sqlite", false},
	}
	for _, c := range cases {
		got, leg := ResolveDBBackend(c.backend, c.legacy, c.def)
		if got != c.want || leg != c.wantLegacy {
			t.Errorf("Resolve(%q,%q)=(%q,%v) want (%q,%v)", c.backend, c.legacy, got, leg, c.want, c.wantLegacy)
		}
	}
}

func TestLoadFromEnvFileDBBackend(t *testing.T) {
	for name, tc := range map[string]struct{ env, want string }{
		"new":    {"MUXCORE_DB_BACKEND=\"postgres\"\n", "postgres"},
		"legacy": {"MUXCORE_PROFILE=\"postgres\"\n", "postgres"},
		"none":   {"INSTALL_RUNTIME=\"host\"\n", "sqlite"},
	} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".env"), []byte(tc.env), 0o600); err != nil {
			t.Fatal(err)
		}
		a, err := LoadFromEnvFile(root)
		if err != nil {
			t.Fatal(err)
		}
		if a.DBBackend != tc.want {
			t.Errorf("%s: DBBackend=%q want %q", name, a.DBBackend, tc.want)
		}
	}
}

func TestEnvKVUsesDBBackendNotProfile(t *testing.T) {
	a := Default(t.TempDir())
	a.DBBackend = "postgres"
	kv := (&Pipeline{Answers: &a}).envKV(&a)
	if kv["MUXCORE_DB_BACKEND"] != "postgres" {
		t.Errorf("MUXCORE_DB_BACKEND=%q", kv["MUXCORE_DB_BACKEND"])
	}
	if _, ok := kv["MUXCORE_PROFILE"]; ok {
		t.Error("installer must not set MUXCORE_PROFILE for core")
	}
}

func TestMigrateEnvFile(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, ".env")
	if err := os.WriteFile(p, []byte("MUXCORE_PROFILE=\"postgres\"\nINSTALL_RUNTIME=\"host\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ok, err := migrateEnvFile(p)
	if err != nil || !ok {
		t.Fatalf("migrate = %v, %v", ok, err)
	}
	b, _ := os.ReadFile(p)
	s := string(b)
	if strings.Contains(s, "MUXCORE_PROFILE") || !strings.Contains(s, `MUXCORE_DB_BACKEND="postgres"`) || !strings.Contains(s, "INSTALL_RUNTIME") {
		t.Fatalf("unexpected .env:\n%s", s)
	}
	if ok, _ := migrateEnvFile(p); ok {
		t.Error("second migrate should be a no-op")
	}
	// security-profile values are left alone
	if err := os.WriteFile(p, []byte("MUXCORE_PROFILE=\"household\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if ok, _ := migrateEnvFile(p); ok {
		t.Error("household must not be migrated")
	}
	// existing new var wins
	if err := os.WriteFile(p, []byte("MUXCORE_PROFILE=\"postgres\"\nMUXCORE_DB_BACKEND=\"sqlite\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if ok, _ := migrateEnvFile(p); !ok {
		t.Fatal("expected migration")
	}
	b, _ = os.ReadFile(p)
	if strings.Contains(string(b), "MUXCORE_PROFILE") || !strings.Contains(string(b), `MUXCORE_DB_BACKEND="sqlite"`) {
		t.Fatalf("unexpected .env:\n%s", b)
	}
}

func TestConfigureMigratesExistingEnv(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, ".env")
	if err := os.WriteFile(p, []byte("MUXCORE_PROFILE=\"sqlite\"\nINSTALL_RUNTIME=\"host\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := Default(root)
	a.Agreed = true
	a.Root = root
	var msgs []string
	pl := &Pipeline{Answers: &a, DryRun: true, Emit: func(pr Progress) { msgs = append(msgs, pr.Text) }}
	if err := pl.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if strings.Contains(string(b), "MUXCORE_PROFILE") || !strings.Contains(string(b), "MUXCORE_DB_BACKEND") {
		t.Fatalf(".env not migrated:\n%s", b)
	}
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "migrated .env") {
			found = true
		}
	}
	if !found {
		t.Errorf("no migration message in %v", msgs)
	}
}
