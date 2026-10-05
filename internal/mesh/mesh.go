// Package mesh holds the installer's side of MuxCore's mesh security:
// the security profile (ADR-0016), enrollment tokens (ADR-0017) and the
// on-disk layout of the core CA and module identities (ADR-0023).
//
// Layout under an install root (host and compose runtimes; see up.sh):
//
//	mesh/                  core's MUXCORE_DATA_DIR (0700)
//	mesh/ca/               core CA key, enrollment ledger, core's server cert
//	                       (MUXCORE_GRPC_CA_CERT_DIR, 0700)
//	mesh/public/ca.crt     public CA exported by core (MUXCORE_CA_EXPORT_DIR);
//	                       every module verifies core with it (MUXCORE_TLS_CA)
//	mesh/id/<module-id>/   one identity dir per module (MUXCORE_TLS_DIR, 0700)
//
// mesh/ is key material, not state, and sits outside data/ on purpose: a
// backup of data/ never contains it (ADR-0023), and compose module
// containers, which mount data/, cannot read the CA key. Losing mesh/ only
// means re-enrolling (fresh CA, empty ledger, same tokens).
package mesh

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Security profiles (ADR-0016 §1).
const (
	ProfileDev       = "dev"
	ProfileHousehold = "household"
)

// Environment variable names shared with core and the module SDK.
const (
	EnvProfile        = "MUXCORE_PROFILE"
	EnvInsecure       = "MUXCORE_INSECURE_DISABLE_TLS"
	EnvEnrollSecret   = "MUXCORE_ENROLL_SECRET" //nolint:gosec // env var name, not a credential
	EnvEnrollTokenPfx = "MUXCORE_ENROLL_TOKEN_" //nolint:gosec // env var name prefix, not a credential
)

// InsecureKeys are every variable that turns plaintext mesh on somewhere in
// the stack (core, SDK modules, admin-ui's deprecated alias). None may be set
// in the household profile.
var InsecureKeys = []string{EnvInsecure, "MUXCORE_DEV_TLS_SKIP", "MUXCORE_GRPC_INSECURE", "ADMIN_UI_INSECURE"}

// TokenPrefix starts every version-2 enrollment token (core internal/enroll).
const TokenPrefix = "mct_2_"

// MinSecretLen is core's minimum MUXCORE_ENROLL_SECRET length.
const MinSecretLen = 16

var moduleIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// secretRe keeps the secret safe in a .env file and in compose
// interpolation (same rule as umbrella _mvp/scripts/gen-enrollment.sh).
var secretRe = regexp.MustCompile(`^[A-Za-z0-9._~+/=-]+$`)

// ValidModuleID reports whether id can be enrolled (core's rule).
func ValidModuleID(id string) error {
	if !moduleIDRe.MatchString(id) {
		return fmt.Errorf("invalid module ID %q: use 1-64 letters, digits, '.', '_' or '-'", id)
	}
	return nil
}

// ValidSecret checks an enrollment secret.
func ValidSecret(secret string) error {
	if len(secret) < MinSecretLen {
		return fmt.Errorf("%s must be at least %d characters", EnvEnrollSecret, MinSecretLen)
	}
	if !secretRe.MatchString(secret) {
		return fmt.Errorf("%s may only contain [A-Za-z0-9._~+/=-]", EnvEnrollSecret)
	}
	return nil
}

// NewSecret returns a fresh random enrollment secret (32 bytes, hex).
func NewSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate %s: %w", EnvEnrollSecret, err)
	}
	return hex.EncodeToString(b), nil
}

// Token returns the single-use enrollment token for id, exactly as core
// computes it (`muxcored enroll token <id>`):
//
//	mct_2_<id>_ + hex(HMAC-SHA256(key = secret, msg = id))
func Token(secret, id string) string {
	h := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = h.Write([]byte(id))
	return TokenPrefix + id + "_" + hex.EncodeToString(h.Sum(nil))
}

// TokenVar is the .env variable holding id's token for the compose runtime:
// MUXCORE_ENROLL_TOKEN_ + id upper-cased with '-' and '.' as '_'.
func TokenVar(id string) string {
	r := strings.NewReplacer("-", "_", ".", "_")
	return EnvEnrollTokenPfx + r.Replace(strings.ToUpper(id))
}

func truthy(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "true" || v == "1"
}

// NormalizeProfile maps a MUXCORE_PROFILE value to dev|household ("" stays
// ""). staging is household's alias; the legacy installer DB selector values
// sqlite|postgres count as unset, as in core.
func NormalizeProfile(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "sqlite", "postgres":
		return "", nil
	case ProfileDev:
		return ProfileDev, nil
	case ProfileHousehold, "staging":
		return ProfileHousehold, nil
	}
	return "", fmt.Errorf("unknown %s %q: use dev or household", EnvProfile, v)
}

// Resolve applies core's rule to an install's settings: an explicit profile
// wins; unset with the insecure flag is dev (ADR-0016 phase-0 inference,
// inferred=true); unset without it is household.
func Resolve(profile, insecure string) (name string, inferred bool, err error) {
	name, err = NormalizeProfile(profile)
	if err != nil || name != "" {
		return name, false, err
	}
	if truthy(insecure) {
		return ProfileDev, true, nil
	}
	return ProfileHousehold, false, nil
}

// Dir is core's data dir for mesh key material (MUXCORE_DATA_DIR).
func Dir(root string) string { return filepath.Join(root, "mesh") }

// CADir is core's CA directory (MUXCORE_GRPC_CA_CERT_DIR).
func CADir(root string) string { return filepath.Join(Dir(root), "ca") }

// CAExportDir is where core exports the public CA certificate.
func CAExportDir(root string) string { return filepath.Join(Dir(root), "public") }

// CACertPath is the exported public CA certificate.
func CACertPath(root string) string { return filepath.Join(CAExportDir(root), "ca.crt") }

// IdentityRoot holds one identity directory per module.
func IdentityRoot(root string) string { return filepath.Join(Dir(root), "id") }

// IdentityDir is module id's identity directory (MUXCORE_TLS_DIR).
func IdentityDir(root, id string) string { return filepath.Join(IdentityRoot(root), id) }

// KeyMaterialDir is the install-root-relative directory a backup must skip
// (ADR-0023).
const KeyMaterialDir = "mesh"

// EnsureDirs creates the mesh layout for root and the given module IDs:
// private dirs 0700, the public CA dir 0755. Existing dirs are tightened to
// those modes.
func EnsureDirs(root string, moduleIDs []string) error {
	type d struct {
		path string
		mode os.FileMode
	}
	dirs := []d{{Dir(root), 0o700}, {CADir(root), 0o700}, {CAExportDir(root), 0o755}, {IdentityRoot(root), 0o700}}
	for _, id := range moduleIDs {
		if err := ValidModuleID(id); err != nil {
			return err
		}
		dirs = append(dirs, d{IdentityDir(root, id), 0o700})
	}
	for _, x := range dirs {
		if err := os.MkdirAll(x.path, x.mode); err != nil {
			return err
		}
		if err := os.Chmod(x.path, x.mode); err != nil {
			return err
		}
	}
	return nil
}
