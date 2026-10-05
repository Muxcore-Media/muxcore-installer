// Package pins embeds versions.env at build time so a single muxcore-setup
// binary carries its own module pin matrix (no separate file to ship or lose).
//
// versions.env stays the single source of truth on disk (scripts/check-pin-matrix.sh
// and PIN-MATRIX.md read it directly); the repo-root versions.env is a symlink into
// this package so `go:embed` can see it without an out-of-module path.
package pins

import (
	_ "embed"
	"regexp"
	"strings"
)

//go:embed versions.env
var raw string

var defaultAssign = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*"?\$\{[A-Za-z0-9_]+:-([^}"]*)\}"?$`)

// Pins holds every value the installer needs from versions.env.
type Pins struct {
	InstallerTag string
	CoreRepo     string
	CoreTag      string
	CoreAssetPfx string
	Modules      map[string]string // repo -> tag
	HelperBins   []string
}

var loaded *Pins

// Load parses the embedded versions.env once and caches the result.
func Load() *Pins {
	if loaded != nil {
		return loaded
	}
	p := &Pins{Modules: map[string]string{}}
	inModules := false
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if inModules {
			if trimmed == `"` {
				inModules = false
				continue
			}
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if eq := strings.Index(trimmed, "="); eq > 0 {
				p.Modules[strings.TrimSpace(trimmed[:eq])] = strings.TrimSpace(trimmed[eq+1:])
			}
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, `MODULES="`) {
			inModules = true
			continue
		}
		if m := defaultAssign.FindStringSubmatch(trimmed); m != nil {
			applyKV(p, m[1], m[2])
			continue
		}
		if eq := strings.Index(trimmed, "="); eq > 0 {
			key := strings.TrimSpace(trimmed[:eq])
			val := strings.Trim(strings.TrimSpace(trimmed[eq+1:]), `"`)
			applyKV(p, key, val)
		}
	}
	loaded = p
	return p
}

func applyKV(p *Pins, key, val string) {
	switch key {
	case "INSTALLER_TAG":
		p.InstallerTag = val
	case "CORE_REPO":
		p.CoreRepo = val
	case "CORE_TAG":
		p.CoreTag = val
	case "CORE_ASSET_PREFIX":
		p.CoreAssetPfx = val
	case "HELPER_BINS":
		p.HelperBins = strings.Fields(val)
	}
}

// Raw returns the exact embedded versions.env text, so callers that lay out
// an install root (internal/assets) can write it out for up.sh to source —
// up.sh still reads versions.env directly rather than duplicating pins in Go.
func Raw() string { return raw }

// ModuleTag returns the pinned release tag for a module/binary name.
func (p *Pins) ModuleTag(name string) string {
	if name == "muxcored" {
		if p.CoreTag != "" {
			return p.CoreTag
		}
		return "v0.6.7"
	}
	if tag, ok := p.Modules[name]; ok {
		return tag
	}
	return "v0.1.0"
}
