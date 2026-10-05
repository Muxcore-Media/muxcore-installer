package wizard

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Muxcore-Media/muxcore-installer/internal/compose"
	"github.com/Muxcore-Media/muxcore-installer/internal/envfile"
	"github.com/Muxcore-Media/muxcore-installer/internal/mesh"
)

// Security profile handling (ADR-0016 phase 1): new installs run the
// household profile — TLS on every mesh hop with core's own CA, every module
// enrolled for its own certificate (ADR-0017). The dev profile (plaintext
// mesh) is an explicit escape hatch: `muxcore-setup --dev`,
// MUXCORE_PROFILE=dev, or `./up.sh --dev` for one run.
//
// Existing installs are never switched silently. An install that runs dev
// today (MUXCORE_PROFILE=dev, or the legacy MUXCORE_INSECURE_DISABLE_TLS=true
// with no profile) keeps dev on a re-run — its binaries and scripts predate
// household and would fail mid-upgrade — and the installer prints the opt-in
// command. Opting in (`muxcore-setup --household` or MUXCORE_PROFILE=household)
// stops the stack, moves bin/ and the managed scripts aside so the pinned,
// household-capable versions are installed, and rewrites .env.

// MigrateHint is the opt-in command printed for installs that stay dev.
const MigrateHint = "to switch this install to the secure household profile, re-run the installer with " +
	"`muxcore-setup --household` (or MUXCORE_PROFILE=household; in the wizard choose Reconfigure or Restart only): " +
	"it stops MuxCore, keeps your data, settings and logins, moves bin/ and the scripts aside " +
	"(bin.pre-household-*, *.pre-household) and installs the pinned versions"

// ParseRequestedProfile turns the operator's explicit choice into a profile
// ("" = no explicit choice). Flags win over MUXCORE_PROFILE; the legacy
// database selector values sqlite|postgres in MUXCORE_PROFILE are not a
// request.
func ParseRequestedProfile(devFlag, householdFlag bool, envProfile string) (string, error) {
	switch {
	case devFlag && householdFlag:
		return "", errors.New("--dev and --household are mutually exclusive")
	case devFlag:
		return mesh.ProfileDev, nil
	case householdFlag:
		return mesh.ProfileHousehold, nil
	}
	return mesh.NormalizeProfile(envProfile)
}

// securityPlan is what stepConfigure does about the security profile.
type securityPlan struct {
	// Profile is the profile the install runs after this run.
	Profile string
	// Existing is the profile of the existing install ("" = no .env yet).
	Existing string
	// Inferred: Existing came from the legacy insecure flag (no profile set).
	Inferred bool
	// Migrate: an existing dev install switches to household now.
	Migrate bool
}

// planSecurity decides the profile. existing is the current .env (nil when
// there is none), choice the existing-install choice, requested the explicit
// request ("" = none).
func planSecurity(existing *envfile.File, choice, requested string) (securityPlan, error) {
	if existing == nil {
		p := requested
		if p == "" {
			p = mesh.ProfileHousehold
		}
		return securityPlan{Profile: p}, nil
	}
	ex, inferred, err := mesh.Resolve(existing.Get(mesh.EnvProfile, ""), existing.Get(mesh.EnvInsecure, ""))
	if err != nil {
		return securityPlan{}, fmt.Errorf("existing .env: %w", err)
	}
	plan := securityPlan{Existing: ex, Inferred: inferred, Profile: ex}
	switch {
	case requested != "":
		plan.Profile = requested
	case choice == "fresh":
		plan.Profile = mesh.ProfileHousehold
	}
	plan.Migrate = ex == mesh.ProfileDev && plan.Profile == mesh.ProfileHousehold
	return plan, nil
}

// applySecurityEnv writes the security settings for profile into f:
//
//   - dev: MUXCORE_PROFILE=dev and MUXCORE_INSECURE_DISABLE_TLS=true (pinned
//     explicitly so ADR-0016 phase 2, which drops the inference, keeps it dev).
//   - household: MUXCORE_PROFILE=household, every insecure flag removed, the
//     enrollment secret generated once (kept on re-runs; .env is 0600) and,
//     for the compose runtime, one MUXCORE_ENROLL_TOKEN_<ID> per module
//     (compose interpolates them; up.sh derives tokens itself).
func applySecurityEnv(f *envfile.File, profile, runtime string, moduleIDs []string) error {
	f.Set(mesh.EnvProfile, profile)
	for k := range f.AsMap() {
		if strings.HasPrefix(k, mesh.EnvEnrollTokenPfx) {
			f.Delete(k)
		}
	}
	if profile == mesh.ProfileDev {
		f.Set(mesh.EnvInsecure, "true")
		return nil
	}
	for _, k := range mesh.InsecureKeys {
		f.Delete(k)
	}
	// Older .env files point the smoke at plain HTTP; core's HTTP port is
	// HTTPS in household, and smoke-fixture.sh picks the scheme itself.
	if u := f.Get("SMOKE_CORE_URL", ""); strings.HasPrefix(u, "http://") {
		f.Delete("SMOKE_CORE_URL")
	}
	secret := f.Get(mesh.EnvEnrollSecret, "")
	if secret == "" {
		s, err := mesh.NewSecret()
		if err != nil {
			return err
		}
		secret = s
	} else if err := mesh.ValidSecret(secret); err != nil {
		return fmt.Errorf("existing .env: %w (remove it to generate a new one)", err)
	}
	f.Set(mesh.EnvEnrollSecret, secret)
	if runtime == "compose" {
		for _, id := range moduleIDs {
			if err := mesh.ValidModuleID(id); err != nil {
				return err
			}
			f.Set(mesh.TokenVar(id), mesh.Token(secret, id))
		}
	}
	return nil
}

// moduleIDs are the mesh module IDs of the selected modules.
func (a *Answers) moduleIDs() []string {
	return compose.ModuleIDs(a.EnabledModules, a.HasPlayback("MuxCore player"))
}

// loadExistingEnv returns the .env at path, or nil when there is none.
func loadExistingEnv(path string) (*envfile.File, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return envfile.Load(path)
}

// planNote is the operator-facing line for plan (empty when nothing to say).
func planNote(plan securityPlan, requested string) (StepKind, string) {
	switch {
	case plan.Migrate:
		return KindLine, "switching this install from the insecure dev profile to household (ADR-0016): " +
			"TLS on every mesh hop, core CA in mesh/ca, one enrolled identity per module in mesh/id"
	case plan.Profile == mesh.ProfileDev && requested == mesh.ProfileDev:
		return KindWarn, "security profile: dev (requested) — the mesh is plaintext and modules are trusted by " +
			"the ID they claim. For development only; never for real data or a reachable network"
	case plan.Profile == mesh.ProfileDev:
		how := "MUXCORE_PROFILE=dev"
		if plan.Inferred {
			how = "MUXCORE_INSECURE_DISABLE_TLS=true (now pinned as MUXCORE_PROFILE=dev)"
		}
		return KindWarn, "security profile: dev, kept from the existing install (" + how + "); " + MigrateHint
	}
	return KindLine, "security profile: household — TLS on every mesh hop, modules enroll with core's CA"
}

// migrationSuffix marks files the household migration moved aside.
const migrationSuffix = ".pre-household"

// migrateToHousehold prepares an existing dev install for household: stop
// the stack with its own (old) up.sh, move bin/ aside so the pinned binaries
// are downloaded again, and move the managed scripts aside so the current
// ones are extracted. Nothing in data/ is touched. Rollback: stop, move
// bin.pre-household-* back to bin/ and the *.pre-household scripts back, set
// MUXCORE_PROFILE=dev in .env.
func (p *Pipeline) migrateToHousehold(root string) error {
	upSh := filepath.Join(root, "up.sh")
	if _, err := os.Stat(upSh); err == nil {
		p.emit(StepConfigure, KindLine, "stopping MuxCore before the switch (./up.sh stop)")
		if err := runQuiet(upSh, root, "stop"); err != nil {
			p.emit(StepConfigure, KindWarn, "./up.sh stop: "+err.Error()+" — continuing")
		}
	}
	stamp := time.Now().Format("20060102-150405")
	bin := filepath.Join(root, "bin")
	if st, err := os.Stat(bin); err == nil && st.IsDir() {
		aside := filepath.Join(root, "bin.pre-household-"+stamp)
		if err := os.Rename(bin, aside); err != nil {
			return fmt.Errorf("move %s aside: %w", bin, err)
		}
		p.emit(StepConfigure, KindLine, "moved the dev-era binaries to "+filepath.Base(aside)+" (the pinned binaries are downloaded again)")
	}
	moved, err := moveManagedScriptsAside(root, migrationSuffix)
	if err != nil {
		return err
	}
	if len(moved) > 0 {
		p.emit(StepConfigure, KindLine, "moved the old scripts aside: "+strings.Join(moved, ", ")+" (*"+migrationSuffix+")")
	}
	return nil
}

// managedFiles are the installer-owned files under an install root that
// the household migration replaces (ExtractInto writes them again).
var managedFiles = []string{
	"up.sh", "bootstrap-auth.sh", "smoke-fixture.sh", "lib/common.sh", "lib/modules.sh",
	".env.example", "versions.env",
}

func moveManagedScriptsAside(root, suffix string) ([]string, error) {
	var moved []string
	for _, rel := range managedFiles {
		src := filepath.Join(root, rel)
		if _, err := os.Lstat(src); err != nil {
			continue
		}
		if err := os.Rename(src, src+suffix); err != nil {
			return moved, fmt.Errorf("move %s aside: %w", rel, err)
		}
		moved = append(moved, rel)
	}
	return moved, nil
}

// runQuiet runs name args in dir and returns an error carrying the tail of
// its output when it fails.
func runQuiet(name, dir string, args ...string) error {
	cmd := exec.Command(name, args...) //nolint:gosec // installer-owned script under the install root
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		tail := strings.TrimSpace(string(out))
		if len(tail) > 400 {
			tail = tail[len(tail)-400:]
		}
		if tail != "" {
			return fmt.Errorf("%w: %s", err, tail)
		}
		return err
	}
	return nil
}
