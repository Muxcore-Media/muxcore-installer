package wizard

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Muxcore-Media/muxcore-installer/internal/envfile"
	"github.com/Muxcore-Media/muxcore-installer/internal/mesh"
)

func TestParseRequestedProfile(t *testing.T) {
	cases := []struct {
		dev, household bool
		env, want      string
		err            bool
	}{
		{false, false, "", "", false},
		{true, false, "", "dev", false},
		{false, true, "", "household", false},
		{false, true, "dev", "household", false}, // flag wins
		{true, true, "", "", true},
		{false, false, "dev", "dev", false},
		{false, false, "staging", "household", false},
		{false, false, "sqlite", "", false}, // legacy DB selector is not a request
		{false, false, "prod", "", true},
	}
	for _, c := range cases {
		got, err := ParseRequestedProfile(c.dev, c.household, c.env)
		if (err != nil) != c.err || got != c.want {
			t.Errorf("ParseRequestedProfile(%v,%v,%q) = %q,%v want %q,err=%v", c.dev, c.household, c.env, got, err, c.want, c.err)
		}
	}
}

func envFrom(t *testing.T, content string) *envfile.File {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := envfile.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestPlanSecurity(t *testing.T) {
	legacyDev := "MUXCORE_INSECURE_DISABLE_TLS=\"true\"\n"
	cases := []struct {
		name      string
		existing  string // "-" = no .env
		choice    string
		requested string
		want      securityPlan
	}{
		{"new install", "-", "", "", securityPlan{Profile: "household"}},
		{"new install --dev", "-", "", "dev", securityPlan{Profile: "dev"}},
		{"legacy dev reconfigure stays dev", legacyDev, "reconfigure", "", securityPlan{Profile: "dev", Existing: "dev", Inferred: true}},
		{"legacy dev restart-only stays dev", legacyDev, "restart-only", "", securityPlan{Profile: "dev", Existing: "dev", Inferred: true}},
		{"legacy dev opts in", legacyDev, "reconfigure", "household", securityPlan{Profile: "household", Existing: "dev", Inferred: true, Migrate: true}},
		{"legacy dev fresh choice", legacyDev, "fresh", "", securityPlan{Profile: "household", Existing: "dev", Inferred: true, Migrate: true}},
		{"explicit dev stays dev", "MUXCORE_PROFILE=\"dev\"\nMUXCORE_INSECURE_DISABLE_TLS=\"true\"\n", "reconfigure", "", securityPlan{Profile: "dev", Existing: "dev"}},
		{"household stays household", "MUXCORE_PROFILE=\"household\"\n", "reconfigure", "", securityPlan{Profile: "household", Existing: "household"}},
		{"household asks for dev", "MUXCORE_PROFILE=\"household\"\n", "reconfigure", "dev", securityPlan{Profile: "dev", Existing: "household"}},
		{"no flag no profile is household", "MVP_ADMIN_USER=\"a\"\n", "reconfigure", "", securityPlan{Profile: "household", Existing: "household"}},
	}
	for _, c := range cases {
		var f *envfile.File
		if c.existing != "-" {
			f = envFrom(t, c.existing)
		}
		got, err := planSecurity(f, c.choice, c.requested)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: plan = %+v, want %+v", c.name, got, c.want)
		}
	}
	if _, err := planSecurity(envFrom(t, "MUXCORE_PROFILE=\"prod\"\n"), "reconfigure", ""); err == nil {
		t.Error("unknown MUXCORE_PROFILE in .env accepted")
	}
}

func TestApplySecurityEnvHousehold(t *testing.T) {
	f := envFrom(t, `MUXCORE_INSECURE_DISABLE_TLS="true"
ADMIN_UI_INSECURE="true"
MUXCORE_GRPC_INSECURE="1"
SMOKE_CORE_URL="http://127.0.0.1:8080"
MUXCORE_ENROLL_TOKEN_STALE="x"
MVP_LIBRARY_ROOT="/keep"
`)
	if err := applySecurityEnv(f, "household", "host", []string{"api-rest"}); err != nil {
		t.Fatal(err)
	}
	m := f.AsMap()
	if m["MUXCORE_PROFILE"] != "household" {
		t.Errorf("MUXCORE_PROFILE = %q", m["MUXCORE_PROFILE"])
	}
	for _, k := range append(append([]string{}, mesh.InsecureKeys...), "SMOKE_CORE_URL", "MUXCORE_ENROLL_TOKEN_STALE", "MUXCORE_ENROLL_TOKEN_API_REST") {
		if _, ok := m[k]; ok {
			t.Errorf("%s still set (host runtime keeps no tokens in .env)", k)
		}
	}
	secret := m["MUXCORE_ENROLL_SECRET"]
	if mesh.ValidSecret(secret) != nil {
		t.Fatalf("secret %q", secret)
	}
	if m["MVP_LIBRARY_ROOT"] != "/keep" {
		t.Error("unrelated key changed")
	}
	// Generated once: a re-run keeps it.
	if err := applySecurityEnv(f, "household", "host", nil); err != nil {
		t.Fatal(err)
	}
	if f.Get("MUXCORE_ENROLL_SECRET", "") != secret {
		t.Error("secret regenerated on re-run")
	}
}

func TestApplySecurityEnvComposeTokens(t *testing.T) {
	f := envFrom(t, "MUXCORE_ENROLL_SECRET=\"0123456789abcdef0123456789abcdef\"\n")
	if err := applySecurityEnv(f, "household", "compose", []string{"api-rest", "media-ui"}); err != nil {
		t.Fatal(err)
	}
	if got := f.Get("MUXCORE_ENROLL_TOKEN_API_REST", ""); got != "mct_2_api-rest_5d337c18a66da96b244b1d5a0d8044eefcfc966bc7e743e65a9d0f8978ac4b2a" {
		t.Errorf("api-rest token = %q", got)
	}
	if got := f.Get("MUXCORE_ENROLL_TOKEN_MEDIA_UI", ""); got != mesh.Token("0123456789abcdef0123456789abcdef", "media-ui") {
		t.Errorf("media-ui token = %q", got)
	}
}

func TestApplySecurityEnvRejectsBadExistingSecret(t *testing.T) {
	f := envFrom(t, "MUXCORE_ENROLL_SECRET=\"short\"\n")
	if err := applySecurityEnv(f, "household", "host", nil); err == nil {
		t.Fatal("short existing secret accepted")
	}
}

func TestApplySecurityEnvDev(t *testing.T) {
	f := envFrom(t, "MUXCORE_ENROLL_TOKEN_API_REST=\"x\"\n")
	if err := applySecurityEnv(f, "dev", "compose", []string{"api-rest"}); err != nil {
		t.Fatal(err)
	}
	m := f.AsMap()
	if m["MUXCORE_PROFILE"] != "dev" || m["MUXCORE_INSECURE_DISABLE_TLS"] != "true" {
		t.Errorf("dev env = %v", m)
	}
	if _, ok := m["MUXCORE_ENROLL_TOKEN_API_REST"]; ok {
		t.Error("dev keeps enrollment tokens")
	}
}

func readEnv(t *testing.T, root string) map[string]string {
	t.Helper()
	f, err := envfile.Load(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	return f.AsMap()
}

func TestPipelineFreshInstallIsHousehold(t *testing.T) {
	root := t.TempDir()
	a := Default(root)
	a.Agreed = true
	var msgs []string
	p := &Pipeline{Answers: &a, DryRun: true, Emit: func(pr Progress) { msgs = append(msgs, pr.Text) }}
	if err := p.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	env := readEnv(t, root)
	if env["MUXCORE_PROFILE"] != "household" || a.SecurityProfile != "household" {
		t.Fatalf("profile %q / %q", env["MUXCORE_PROFILE"], a.SecurityProfile)
	}
	for _, k := range mesh.InsecureKeys {
		if _, ok := env[k]; ok {
			t.Errorf("%s set in a household .env", k)
		}
	}
	if mesh.ValidSecret(env["MUXCORE_ENROLL_SECRET"]) != nil {
		t.Errorf("no valid enrollment secret: %q", env["MUXCORE_ENROLL_SECRET"])
	}
	if _, ok := env["SMOKE_CORE_URL"]; ok {
		t.Error("SMOKE_CORE_URL pinned to plain HTTP")
	}
	st, err := os.Stat(filepath.Join(root, ".env"))
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf(".env mode: %v %v", st.Mode().Perm(), err)
	}
	for _, d := range []string{mesh.CADir(root), mesh.IdentityDir(root, "api-rest"), mesh.IdentityDir(root, "media-ui")} {
		if st, err := os.Stat(d); err != nil || st.Mode().Perm() != 0o700 {
			t.Errorf("%s: %v", d, err)
		}
	}
	view, _ := os.ReadFile(filepath.Join(root, "run", "VIEW-ME.txt"))
	if !strings.Contains(string(view), "https://127.0.0.1:8080/health") || !strings.Contains(string(view), "never back it up") {
		t.Errorf("VIEW-ME.txt:\n%s", view)
	}
	if !containsAny(msgs, "security profile: household") {
		t.Errorf("no household note in %v", msgs)
	}
}

func TestPipelineDevRequested(t *testing.T) {
	root := t.TempDir()
	a := Default(root)
	a.Agreed = true
	a.RequestedProfile = "dev"
	if err := (&Pipeline{Answers: &a, DryRun: true}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	env := readEnv(t, root)
	if env["MUXCORE_PROFILE"] != "dev" || env["MUXCORE_INSECURE_DISABLE_TLS"] != "true" {
		t.Fatalf("dev .env = %v", env)
	}
	if _, err := os.Stat(mesh.Dir(root)); !os.IsNotExist(err) {
		t.Error("dev install created mesh/")
	}
}

// writeLegacyDevInstall lays out an install made by an installer before
// household: insecure flag, no profile, old up.sh, binaries in bin/.
func writeLegacyDevInstall(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	env := `INSTALL_RUNTIME="host"
MUXCORE_INSECURE_DISABLE_TLS="true"
MUXCORE_LIBRARIES="Movies"
MVP_LIBRARY_ROOT="/keep/movies"
MVP_ADMIN_USER="admin"
MVP_ADMIN_PASSWORD="keep-secret-pass"
SMOKE_CORE_URL="http://127.0.0.1:8080"
`
	must(t, os.WriteFile(filepath.Join(root, ".env"), []byte(env), 0o600))
	// Old up.sh: records how it was called.
	must(t, os.WriteFile(filepath.Join(root, "up.sh"), []byte("#!/bin/sh\necho \"$@\" >> \"$(dirname \"$0\")/up-calls.log\"\n"), 0o755))
	must(t, os.MkdirAll(filepath.Join(root, "bin"), 0o755))
	must(t, os.WriteFile(filepath.Join(root, "bin", "muxcored"), []byte("old"), 0o755))
	must(t, os.MkdirAll(filepath.Join(root, "data", "movies"), 0o755))
	must(t, os.WriteFile(filepath.Join(root, "data", "movies", "movies.db"), []byte("state"), 0o600))
	return root
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func containsAny(msgs []string, sub string) bool {
	for _, m := range msgs {
		if strings.Contains(m, sub) {
			return true
		}
	}
	return false
}

func TestExistingDevInstallStaysDevWithoutOptIn(t *testing.T) {
	root := writeLegacyDevInstall(t)
	a := Default(root)
	a.Agreed = true
	a.ExistingChoice = "reconfigure"
	a.Libraries = []string{"Movies"}
	a.LibraryPaths = map[string]string{"movies": "/keep/movies"}
	var msgs []string
	p := &Pipeline{Answers: &a, Emit: func(pr Progress) { msgs = append(msgs, pr.Text) }}
	p.Answers.EnabledModules = []string{"muxcored", "api-rest"}
	if err := p.stepConfigure(); err != nil {
		t.Fatal(err)
	}
	env := readEnv(t, root)
	if env["MUXCORE_PROFILE"] != "dev" || env["MUXCORE_INSECURE_DISABLE_TLS"] != "true" {
		t.Fatalf("existing dev install changed profile: %v", env)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", "muxcored")); err != nil {
		t.Error("bin/ moved without opt-in")
	}
	if _, err := os.Stat(filepath.Join(root, "up-calls.log")); !os.IsNotExist(err) {
		t.Error("stack stopped without opt-in")
	}
	if !containsAny(msgs, "--household") {
		t.Errorf("no opt-in hint in %v", msgs)
	}
}

func TestExistingDevInstallMigratesOnOptIn(t *testing.T) {
	root := writeLegacyDevInstall(t)
	a := Default(root)
	a.Agreed = true
	a.ExistingChoice = "reconfigure"
	a.RequestedProfile = "household"
	a.Libraries = []string{"Movies"}
	a.LibraryPaths = map[string]string{"movies": "/keep/movies"}
	a.AdminPass = "keep-secret-pass"
	a.EnabledModules = []string{"muxcored", "api-rest"}
	p := &Pipeline{Answers: &a, Emit: func(Progress) {}}
	if err := p.stepConfigure(); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(filepath.Join(root, "up-calls.log"))
	if strings.TrimSpace(string(calls)) != "stop" {
		t.Errorf("old up.sh calls = %q, want stop", calls)
	}
	if _, err := os.Stat(filepath.Join(root, "bin")); !os.IsNotExist(err) {
		t.Error("bin/ not moved aside (the pinned binaries must be fetched again)")
	}
	aside, _ := filepath.Glob(filepath.Join(root, "bin.pre-household-*", "muxcored"))
	if len(aside) != 1 {
		t.Errorf("old binaries not kept for rollback: %v", aside)
	}
	if _, err := os.Stat(filepath.Join(root, "up.sh"+migrationSuffix)); err != nil {
		t.Error("old up.sh not kept for rollback")
	}
	newUp, _ := os.ReadFile(filepath.Join(root, "up.sh"))
	if !strings.Contains(string(newUp), "resolve_security_profile") {
		t.Error("current up.sh not extracted")
	}
	env := readEnv(t, root)
	if env["MUXCORE_PROFILE"] != "household" {
		t.Errorf("profile %q", env["MUXCORE_PROFILE"])
	}
	for _, k := range []string{"MUXCORE_INSECURE_DISABLE_TLS", "SMOKE_CORE_URL"} {
		if _, ok := env[k]; ok {
			t.Errorf("%s survived the migration", k)
		}
	}
	if mesh.ValidSecret(env["MUXCORE_ENROLL_SECRET"]) != nil {
		t.Error("no enrollment secret after migration")
	}
	if env["MVP_LIBRARY_ROOT"] != "/keep/movies" || env["MVP_ADMIN_PASSWORD"] != "keep-secret-pass" {
		t.Errorf("settings lost: %v", env)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "data", "movies", "movies.db")); string(b) != "state" {
		t.Error("module data touched")
	}
}

func TestDryRunNeverMigrates(t *testing.T) {
	root := writeLegacyDevInstall(t)
	a := Default(root)
	a.Agreed = true
	a.ExistingChoice = "reconfigure"
	a.RequestedProfile = "household"
	if err := (&Pipeline{Answers: &a, DryRun: true}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", "muxcored")); err != nil {
		t.Error("dry run moved bin/")
	}
	if env := readEnv(t, root); env["MUXCORE_PROFILE"] != "dev" {
		t.Errorf("dry run switched the profile: %v", env)
	}
}

func TestRestartOnlyOptInChangesOnlySecurityKeys(t *testing.T) {
	root := writeLegacyDevInstall(t)
	a := Default(root)
	a.Agreed = true
	a.ExistingChoice = "restart-only"
	a.RequestedProfile = "household"
	p := &Pipeline{Answers: &a, DryRun: false, Emit: func(Progress) {}}
	loaded, err := LoadFromEnvFile(root)
	must(t, err)
	loaded.ExistingChoice, loaded.RequestedProfile = "restart-only", "household"
	loaded.EnabledModules = []string{"muxcored", "api-rest"}
	p.Answers = loaded
	if err := p.stepConfigure(); err != nil {
		t.Fatal(err)
	}
	env := readEnv(t, root)
	if env["MUXCORE_PROFILE"] != "household" || env["MVP_ADMIN_PASSWORD"] != "keep-secret-pass" || env["MVP_LIBRARY_ROOT"] != "/keep/movies" {
		t.Errorf("restart-only opt-in .env = %v", env)
	}
	if _, ok := env["MUXCORE_INSECURE_DISABLE_TLS"]; ok {
		t.Error("insecure flag kept")
	}
}

func TestHealthClientHousehold(t *testing.T) {
	root := t.TempDir()
	a := &Answers{Root: root, SecurityProfile: "household"}
	if _, err := healthClient(a); err == nil {
		t.Fatal("household health client without core's CA")
	}
	must(t, mesh.EnsureDirs(root, nil))
	must(t, os.WriteFile(mesh.CACertPath(root), selfSignedPEM(t), 0o644))
	c, err := healthClient(a)
	if err != nil || c.Transport == nil {
		t.Fatalf("household health client: %v", err)
	}
	dev := &Answers{Root: root, SecurityProfile: "dev"}
	if c, err := healthClient(dev); err != nil || c.Transport != nil {
		t.Fatalf("dev health client: %v", err)
	}
}

func selfSignedPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test CA"},
		NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	must(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
