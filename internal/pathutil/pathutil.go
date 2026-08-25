// Package pathutil resolves and suggests filesystem paths for the wizard.
// Ported from lib/paths.sh + resolve_install_dir in lib/common.sh.
package pathutil

import (
	"os"
	"path/filepath"
	"runtime"
)

// Resolve expands ~ and makes a path absolute without requiring it to exist.
func Resolve(raw string) string {
	if raw == "" {
		return raw
	}
	home, _ := os.UserHomeDir()
	if raw == "~" {
		return home
	}
	if len(raw) >= 2 && raw[:2] == "~/" {
		raw = filepath.Join(home, raw[2:])
	}
	if !filepath.IsAbs(raw) {
		if wd, err := os.Getwd(); err == nil {
			raw = filepath.Join(wd, raw)
		}
	}
	return filepath.Clean(raw)
}

// EnsureWritableDir creates dir (and parents) and confirms it is writable.
func EnsureWritableDir(dir string) error {
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		return &os.PathError{Op: "ensure", Path: dir, Err: os.ErrExist}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	probe := filepath.Join(dir, ".muxcore-write-test")
	f, err := os.Create(probe)
	if err != nil {
		return err
	}
	f.Close()
	os.Remove(probe)
	return nil
}

// InstallDefault is the always-safe, always-writable default install root.
func InstallDefault() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "muxcore")
}

// InstallRecommended prefers /opt/muxcore when writable.
func InstallRecommended() string {
	if writable("/opt") || writable("/opt/muxcore") {
		return "/opt/muxcore"
	}
	return InstallDefault()
}

// MediaDefault is the always-safe default media root.
func MediaDefault() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Media")
}

// MediaRecommended prefers common mount points / OS folders when present.
func MediaRecommended() string {
	if dirExists("/mnt/media") {
		return "/mnt/media"
	}
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		if p := filepath.Join(home, "Movies"); dirExists(p) {
			return p
		}
	}
	if p := filepath.Join(home, "Videos"); dirExists(p) {
		return p
	}
	return MediaDefault()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func writable(p string) bool {
	info, err := os.Stat(p)
	if err != nil || !info.IsDir() {
		return false
	}
	probe := filepath.Join(p, ".muxcore-write-test")
	f, err := os.Create(probe)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(probe)
	return true
}
