package mesh

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Fixtures printed by core's own CLI (core v0.6.15, internal/enroll):
//
//	MUXCORE_ENROLL_SECRET=<secret> muxcored enroll token <id>
//
// and cross-checked with Python's hmac module.
func TestTokenMatchesCore(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	cases := []struct{ secret, id, want string }{
		{secret, "api-rest", "mct_2_api-rest_5d337c18a66da96b244b1d5a0d8044eefcfc966bc7e743e65a9d0f8978ac4b2a"},
		{secret, "media-movies", "mct_2_media-movies_ca848516112fbad45dae3ddebdf686aa189829bab500081b63403c4e324feebf"},
		{secret, "a", "mct_2_a_aa2187723645a252148bbf3aad0e20d4c3a2861235bb65a264967af9d46faec2"},
		{"s3cret-with.special~chars+/=", "auth-local", "mct_2_auth-local_f18f05b3e8ca2c6e6779bb8f18cb4ca81faaaed3aa1904e3421f0eb06f035e75"},
		// core trims the secret before use
		{"  " + secret + "\n", "api-rest", "mct_2_api-rest_5d337c18a66da96b244b1d5a0d8044eefcfc966bc7e743e65a9d0f8978ac4b2a"},
	}
	for _, c := range cases {
		if got := Token(c.secret, c.id); got != c.want {
			t.Errorf("Token(%q, %q) = %s, want %s", c.secret, c.id, got, c.want)
		}
	}
}

func TestTokenVar(t *testing.T) {
	for id, want := range map[string]string{
		"api-rest":       "MUXCORE_ENROLL_TOKEN_API_REST",
		"media-ui":       "MUXCORE_ENROLL_TOKEN_MEDIA_UI",
		"call.policy-x1": "MUXCORE_ENROLL_TOKEN_CALL_POLICY_X1",
	} {
		if got := TokenVar(id); got != want {
			t.Errorf("TokenVar(%q) = %s, want %s", id, got, want)
		}
	}
}

func TestNewSecretIsValid(t *testing.T) {
	a, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewSecret()
	if a == b {
		t.Fatal("two secrets are equal")
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(a) {
		t.Fatalf("secret %q is not 64 hex chars", a)
	}
	if err := ValidSecret(a); err != nil {
		t.Fatal(err)
	}
}

func TestValidSecret(t *testing.T) {
	if ValidSecret("short") == nil {
		t.Error("short secret accepted")
	}
	if ValidSecret("0123456789abcdef with space") == nil {
		t.Error("secret with a space accepted")
	}
	if ValidSecret("0123456789abcdef$x") == nil {
		t.Error("secret with $ accepted")
	}
}

func TestValidModuleID(t *testing.T) {
	for _, ok := range []string{"api-rest", "media-ui", "a", "x.y_z"} {
		if err := ValidModuleID(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-x", "a b", "a/b"} {
		if ValidModuleID(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestResolve(t *testing.T) {
	cases := []struct {
		profile, insecure string
		want              string
		inferred, err     bool
	}{
		{"", "", ProfileHousehold, false, false},
		{"", "true", ProfileDev, true, false},
		{"", "1", ProfileDev, true, false},
		{"", "false", ProfileHousehold, false, false},
		{"dev", "", ProfileDev, false, false},
		{"household", "", ProfileHousehold, false, false},
		{"staging", "", ProfileHousehold, false, false},
		{"Household", "true", ProfileHousehold, false, false}, // explicit wins; core then refuses the flag
		{"sqlite", "true", ProfileDev, true, false},           // legacy DB selector counts as unset
		{"postgres", "", ProfileHousehold, false, false},
		{"prod", "", "", false, true},
	}
	for _, c := range cases {
		got, inferred, err := Resolve(c.profile, c.insecure)
		if (err != nil) != c.err {
			t.Errorf("Resolve(%q,%q) err = %v", c.profile, c.insecure, err)
			continue
		}
		if got != c.want || inferred != c.inferred {
			t.Errorf("Resolve(%q,%q) = %q,%v want %q,%v", c.profile, c.insecure, got, inferred, c.want, c.inferred)
		}
	}
}

func TestEnsureDirsModes(t *testing.T) {
	root := t.TempDir()
	if err := EnsureDirs(root, []string{"api-rest", "media-ui"}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{
		Dir(root):                     0o700,
		CADir(root):                   0o700,
		CAExportDir(root):             0o755,
		IdentityRoot(root):            0o700,
		IdentityDir(root, "api-rest"): 0o700,
		IdentityDir(root, "media-ui"): 0o700,
	} {
		st, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := st.Mode().Perm(); got != want {
			t.Errorf("%s mode %o, want %o", path, got, want)
		}
	}
	if !strings.HasPrefix(CACertPath(root), filepath.Join(root, KeyMaterialDir)+string(filepath.Separator)) {
		t.Errorf("CA cert %s is outside %s", CACertPath(root), KeyMaterialDir)
	}
	if EnsureDirs(root, []string{"../escape"}) == nil {
		t.Error("EnsureDirs accepted a path-like module ID")
	}
}
