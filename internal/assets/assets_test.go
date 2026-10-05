package assets

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Muxcore-Media/muxcore-installer/internal/mesh"
)

// runCommon sources lib/common.sh in bash and runs script; it returns
// stdout. Tests skip when bash is unavailable.
func runCommon(t *testing.T, env []string, script string) (string, error) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	lib, err := filepath.Abs(filepath.Join("lib", "common.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bash, "-c", `set -euo pipefail; source "$COMMON_SH"; `+script) //nolint:gosec // test-controlled script
	// HOME stays real: python3 may be a toolchain-manager shim that needs it.
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "COMMON_SH=" + lib, "HOME=" + os.Getenv("HOME")}, env...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Logf("stderr: %s", stderr.String())
	}
	return strings.TrimSpace(string(out)), err
}

// The token up.sh hands each module must be byte-identical to core's
// (mesh.Token is pinned to core's CLI output in internal/mesh).
func TestShellEnrollTokenMatchesGo(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	ids := []string{"api-rest", "media-ui", "call-policy-default", "x.y_z"}
	for _, impl := range []string{"python3", "openssl"} {
		if _, err := exec.LookPath(impl); err != nil {
			t.Logf("skip %s: not installed", impl)
			continue
		}
		for _, id := range ids {
			got, err := runCommon(t, []string{"MESH_HMAC=" + impl}, `mesh_enroll_token "`+secret+`" "`+id+`"`)
			if err != nil {
				t.Fatalf("%s %s: %v", impl, id, err)
			}
			if want := mesh.Token(secret, id); got != want {
				t.Errorf("%s token for %s = %s, want %s", impl, id, got, want)
			}
		}
	}
}

func TestShellEnrollTokenRejectsBadID(t *testing.T) {
	if _, err := runCommon(t, nil, `mesh_enroll_token 0123456789abcdef0123456789abcdef "../x"`); err == nil {
		t.Fatal("path-like module ID accepted")
	}
}

func TestShellResolveSecurityProfile(t *testing.T) {
	cases := []struct {
		env   []string
		force string
		want  string
		fail  bool
	}{
		{nil, "0", "household", false},
		{[]string{"MUXCORE_INSECURE_DISABLE_TLS=true"}, "0", "dev", false},
		{[]string{"MUXCORE_INSECURE_DISABLE_TLS=1"}, "0", "dev", false},
		{[]string{"MUXCORE_PROFILE=dev"}, "0", "dev", false},
		{[]string{"MUXCORE_PROFILE=Household"}, "0", "household", false},
		{[]string{"MUXCORE_PROFILE=staging"}, "0", "household", false},
		{[]string{"MUXCORE_PROFILE=household", "MUXCORE_INSECURE_DISABLE_TLS=true"}, "0", "household", false},
		{[]string{"MUXCORE_PROFILE=sqlite", "MUXCORE_INSECURE_DISABLE_TLS=true"}, "0", "dev", false},
		{[]string{"MUXCORE_PROFILE=household"}, "1", "dev", false}, // ./up.sh --dev
		{[]string{"MUXCORE_PROFILE=prod"}, "0", "", true},
	}
	for _, c := range cases {
		got, err := runCommon(t, c.env, `resolve_security_profile `+c.force+`; echo "$SEC_PROFILE"`)
		if (err != nil) != c.fail {
			t.Errorf("%v force=%s: err=%v", c.env, c.force, err)
			continue
		}
		if !c.fail && got != c.want {
			t.Errorf("%v force=%s: profile %q, want %q", c.env, c.force, got, c.want)
		}
	}
}

func TestShellEnsureEnrollSecretGeneratesOnceAndKeeps(t *testing.T) {
	dir := t.TempDir()
	envf := filepath.Join(dir, ".env")
	if err := os.WriteFile(envf, []byte("MVP_ADMIN_USER=\"admin\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := runCommon(t, []string{"ENVF=" + envf}, `ensure_enroll_secret "$ENVF" ""`)
	if err != nil {
		t.Fatal(err)
	}
	if mesh.ValidSecret(first) != nil || len(first) != 64 {
		t.Fatalf("generated secret %q", first)
	}
	st, err := os.Stat(envf)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf(".env mode %o, want 600", st.Mode().Perm())
	}
	data, _ := os.ReadFile(envf)
	if !strings.Contains(string(data), "MUXCORE_ENROLL_SECRET="+first) || !strings.Contains(string(data), "MVP_ADMIN_USER") {
		t.Fatalf(".env after generation:\n%s", data)
	}
	// A later run passes the stored value and gets it back unchanged.
	again, err := runCommon(t, []string{"ENVF=" + envf, "S=" + first}, `ensure_enroll_secret "$ENVF" "$S"`)
	if err != nil || again != first {
		t.Fatalf("second run = %q, %v; want the stored secret", again, err)
	}
	if _, err := runCommon(t, []string{"ENVF=" + envf}, `ensure_enroll_secret "$ENVF" "short"`); err == nil {
		t.Error("short secret accepted")
	}
}

// Nothing the installer ships may turn the insecure flag on unconditionally.
func TestShippedScriptsHaveNoUnconditionalInsecureFlag(t *testing.T) {
	for _, f := range []string{"up.sh", "env.example", "smoke-fixture.sh", "bootstrap-auth.sh"} {
		data, err := files.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			// Only top-level lines run unconditionally; the dev branches
			// (indented) may set the flag.
			if strings.HasPrefix(trimmed, "#") || line != strings.TrimLeft(line, " \t") {
				continue
			}
			if strings.Contains(trimmed, "MUXCORE_INSECURE_DISABLE_TLS=true") || strings.Contains(trimmed, "ADMIN_UI_INSECURE=true") {
				t.Errorf("%s:%d sets the insecure flag unconditionally: %s", f, i+1, trimmed)
			}
		}
	}
	ex, _ := files.ReadFile("env.example")
	if !strings.Contains(string(ex), "\nMUXCORE_PROFILE=household\n") {
		t.Error("env.example does not default to MUXCORE_PROFILE=household")
	}
}
