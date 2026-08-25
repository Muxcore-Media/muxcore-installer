// Package assets embeds the operational shell scripts and config templates
// muxcore-setup needs at runtime (up.sh, bootstrap-auth.sh, smoke-fixture.sh,
// their lib/ helpers, muxcore.json, .env.example, and the default policies),
// so the compiled binary is self-contained: point it at an empty folder and
// it lays out everything it needs before running the pipeline.
//
// Every embedded file here is a symlink back to the canonical copy elsewhere
// in the repo (go:embed cannot reach outside this package's directory) — edit
// the real files, not these symlinks.
package assets

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/Muxcore-Media/muxcore-installer/internal/pins"
)

//go:embed up.sh bootstrap-auth.sh smoke-fixture.sh muxcore.json env.example lib policies
var files embed.FS

// layout maps each embedded path to where it belongs under an install root.
var layout = map[string]string{
	"up.sh":                        "up.sh",
	"bootstrap-auth.sh":            "bootstrap-auth.sh",
	"smoke-fixture.sh":             "smoke-fixture.sh",
	"muxcore.json":                 "muxcore.json",
	"env.example":                  ".env.example",
	"lib/common.sh":                "lib/common.sh",
	"lib/modules.sh":               "lib/modules.sh",
	"policies/call-policy.yaml":    "policies/call-policy.yaml",
	"policies/publish-policy.yaml": "policies/publish-policy.yaml",
}

var executable = map[string]bool{
	"up.sh": true, "bootstrap-auth.sh": true, "smoke-fixture.sh": true,
}

// ExtractInto writes every embedded operational asset into root, skipping
// any file that already exists so a re-run never clobbers local edits.
func ExtractInto(root string) error {
	// up.sh sources this directly; it isn't a symlink target inside this
	// package (it lives with the pin parser), so write it from pins.Raw().
	versionsPath := filepath.Join(root, "versions.env")
	if _, err := os.Stat(versionsPath); os.IsNotExist(err) {
		if err := os.WriteFile(versionsPath, []byte(pins.Raw()), 0o644); err != nil {
			return err
		}
	}
	for src, rel := range layout {
		dest := filepath.Join(root, rel)
		if _, err := os.Stat(dest); err == nil {
			continue
		}
		data, err := files.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if executable[src] {
			mode = 0o755
		}
		if err := os.WriteFile(dest, data, mode); err != nil {
			return err
		}
	}
	return nil
}
