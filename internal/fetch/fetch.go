// Package fetch orchestrates downloading module release binaries into bin/,
// replacing install.sh with native Go so the wizard can report real per-file
// progress instead of a spinner.
package fetch

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Muxcore-Media/muxcore-installer/internal/ghrelease"
	"github.com/Muxcore-Media/muxcore-installer/internal/modules"
	"github.com/Muxcore-Media/muxcore-installer/internal/pins"
)

// OSArch returns the linux|darwin amd64|arm64 pair install.sh used to key off.
func OSArch() (string, string, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	switch osName {
	case "linux", "darwin":
	default:
		return "", "", fmt.Errorf("unsupported OS: %s (need linux or darwin)", osName)
	}
	switch arch {
	case "amd64", "arm64":
	default:
		return "", "", fmt.Errorf("unsupported arch: %s (need amd64 or arm64)", arch)
	}
	return osName, arch, nil
}

// Event is streamed back to the caller (TUI or plain logger) for every
// meaningful step, so the UI can render a checklist + progress bar and the
// bottom viewport can show raw detail lines.
type Event struct {
	Module string
	Kind   EventKind
	Detail string
	Read   int64
	Total  int64
	Err    error
}

type EventKind int

const (
	EventStart EventKind = iota
	EventProgress
	EventDetail
	EventDone
	EventSkipped
	EventMissing
)

// Options configures a fetch run.
type Options struct {
	Root            string   // installer root (contains bin/, cache/)
	Modules         []string // resolved module/binary names to fetch
	LabBinDirs      []string // optional unpublished-binary fallbacks (dev only)
	FetchMediaUIApp bool     // download media-ui-app dist-app when MuxCore player selected
	OnEvent         func(Event)
}

// Result summarizes what happened across the whole module list.
type Result struct {
	Missing   []string
	LabCopied []string
}

func emit(o Options, e Event) {
	if o.OnEvent != nil {
		o.OnEvent(e)
	}
}

// Run downloads every requested module binary (and helper CLIs) into root/bin.
// Missing platform modules are hard failures — the install must not proceed.
func Run(o Options) (Result, error) {
	res := Result{}
	p := pins.Load()
	osName, arch, err := OSArch()
	if err != nil {
		return res, err
	}
	bin := filepath.Join(o.Root, "bin")
	cache := filepath.Join(o.Root, "cache", "releases")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return res, err
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return res, err
	}

	fetchOne := func(name string) error {
		dest := filepath.Join(bin, name)
		if isExecutable(dest) {
			emit(o, Event{Module: name, Kind: EventSkipped, Detail: "already present"})
			return nil
		}
		repo := modules.RepoForBin(name)
		tag := p.ModuleTag(name)
		emit(o, Event{Module: name, Kind: EventStart, Detail: fmt.Sprintf("%s@%s", repo, tag)})
		if err := tryReleaseAsset(o, cache, bin, repo, tag, name, osName, arch); err == nil {
			emit(o, Event{Module: name, Kind: EventDone})
			return nil
		}
		if name == "muxcored" && p.CoreRepo != "" {
			if err := tryReleaseAsset(o, cache, bin, p.CoreRepo, p.CoreTag, "muxcored", osName, arch); err == nil {
				emit(o, Event{Module: name, Kind: EventDone})
				return nil
			}
		}
		if copyFromLab(o.LabBinDirs, bin, name) {
			res.LabCopied = append(res.LabCopied, name)
			emit(o, Event{Module: name, Kind: EventDone, Detail: "copied from lab bin/"})
			return nil
		}
		res.Missing = append(res.Missing, fmt.Sprintf("%s@%s", name, tag))
		emit(o, Event{Module: name, Kind: EventMissing, Detail: fmt.Sprintf("no release binary for %s@%s", repo, tag)})
		return fmt.Errorf("missing required binary: %s (release %s@%s)", name, repo, tag)
	}

	for _, m := range o.Modules {
		if err := fetchOne(m); err != nil {
			return res, err
		}
	}
	for _, h := range p.HelperBins {
		if err := fetchHelper(o, cache, bin, h, osName, arch, &res); err != nil {
			return res, err
		}
	}
	if o.FetchMediaUIApp {
		if err := fetchMediaUIAppDist(o, cache, osName, arch); err != nil {
			return res, err
		}
	}
	return res, nil
}

func fetchHelper(o Options, cache, bin, name, osName, arch string, res *Result) error {
	dest := filepath.Join(bin, name)
	if isExecutable(dest) {
		emit(o, Event{Module: name, Kind: EventSkipped, Detail: "already present"})
		return nil
	}
	emit(o, Event{Module: name, Kind: EventStart, Detail: "helper CLI"})
	var err error
	switch name {
	case "authctl":
		err = tryReleaseAsset(o, cache, bin, "auth-local", pins.Load().ModuleTag("auth-local"), name, osName, arch)
	case "gettoken":
		err = tryReleaseAsset(o, cache, bin, "muxcorectl-cli", "v0.1.0", name, osName, arch)
	default:
		err = fmt.Errorf("unknown helper %s", name)
	}
	if err == nil {
		emit(o, Event{Module: name, Kind: EventDone})
		return nil
	}
	if copyFromLab(o.LabBinDirs, bin, name) {
		res.LabCopied = append(res.LabCopied, name)
		emit(o, Event{Module: name, Kind: EventDone, Detail: "copied from lab bin/"})
		return nil
	}
	res.Missing = append(res.Missing, name)
	emit(o, Event{Module: name, Kind: EventMissing, Detail: "missing helper " + name})
	return fmt.Errorf("missing required helper binary: %s", name)
}

func fetchMediaUIAppDist(o Options, cache, osName, arch string) error {
	p := pins.Load()
	repo := "media-ui-app"
	tag := p.ModuleTag("media-ui-app")
	distDir := filepath.Join(o.Root, "dist-app")
	if indexExists(filepath.Join(distDir, "index.html")) {
		emit(o, Event{Module: "dist-app", Kind: EventSkipped, Detail: "already present"})
		return nil
	}
	ver := strings.TrimPrefix(tag, "v")
	candidates := []string{
		fmt.Sprintf("dist-app_%s_%s_%s.tar.gz", ver, osName, arch),
		fmt.Sprintf("dist-app_%s.tar.gz", ver),
	}
	emit(o, Event{Module: "dist-app", Kind: EventStart, Detail: fmt.Sprintf("%s@%s", repo, tag)})
	for _, asset := range candidates {
		tarball := filepath.Join(cache, fmt.Sprintf("%s-%s-%s", repo, tag, asset))
		sumsPath := filepath.Join(cache, fmt.Sprintf("%s-%s-SHA256SUMS", repo, tag))
		if !isFile(tarball) {
			progress := func(read, total int64) {
				emit(o, Event{Module: "dist-app", Kind: EventProgress, Read: read, Total: total, Detail: asset})
			}
			code, err := ghrelease.DownloadReleaseAsset(repo, tag, asset, tarball, progress)
			if err != nil || code != 200 {
				emit(o, Event{Module: "dist-app", Kind: EventDetail, Detail: fmt.Sprintf("%s: HTTP %d", asset, code)})
				continue
			}
			emit(o, Event{Module: "dist-app", Kind: EventDetail, Detail: "downloaded " + asset})
		}
		if err := ghrelease.DownloadSHA256SUMS(repo, tag, sumsPath); err != nil {
			return fmt.Errorf("media-ui-app SHA256SUMS: %w", err)
		}
		if err := ghrelease.VerifySHA256SUMS(sumsPath, tarball); err != nil {
			return fmt.Errorf("media-ui-app dist checksum: %w", err)
		}
		if err := extractDistApp(tarball, distDir); err != nil {
			emit(o, Event{Module: "dist-app", Kind: EventDetail, Detail: err.Error()})
			continue
		}
		emit(o, Event{Module: "dist-app", Kind: EventDone, Detail: distDir})
		return nil
	}
	return fmt.Errorf("no usable dist-app asset for %s@%s", repo, tag)
}

func extractDistApp(tarball, destDir string) error {
	if err := os.RemoveAll(destDir); err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	f, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		// Tarballs may wrap files in a dist-app/ prefix or ship flat.
		name = strings.TrimPrefix(name, "dist-app/")
		if name == "" || name == "dist-app" {
			continue
		}
		target := filepath.Join(destDir, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
	if !indexExists(filepath.Join(destDir, "index.html")) {
		return fmt.Errorf("tarball %s did not contain dist-app/index.html", tarball)
	}
	return nil
}

func indexExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// tryReleaseAsset downloads and verifies a module release tarball.
func tryReleaseAsset(o Options, cache, bin, repo, tag, binName, osName, arch string) error {
	ver := tag
	if len(ver) > 0 && ver[0] == 'v' {
		ver = ver[1:]
	}
	candidates := []string{
		fmt.Sprintf("%s_%s_%s_%s.tar.gz", binName, ver, osName, arch),
		fmt.Sprintf("%s_%s_%s.tar.gz", binName, osName, arch),
	}
	for _, asset := range candidates {
		dest := filepath.Join(cache, fmt.Sprintf("%s-%s-%s", repo, tag, asset))
		sumsPath := filepath.Join(cache, fmt.Sprintf("%s-%s-SHA256SUMS", repo, tag))
		if !isFile(dest) {
			progress := func(read, total int64) {
				emit(o, Event{Module: binName, Kind: EventProgress, Read: read, Total: total, Detail: asset})
			}
			code, err := ghrelease.DownloadReleaseAsset(repo, tag, asset, dest, progress)
			if err != nil || code != 200 {
				if code == 401 || code == 403 || code == 404 {
					emit(o, Event{Module: binName, Kind: EventDetail, Detail: fmt.Sprintf("%s: HTTP %d (private or missing)", asset, code)})
				}
				continue
			}
			emit(o, Event{Module: binName, Kind: EventDetail, Detail: "downloaded " + asset})
		}
		if err := ghrelease.DownloadSHA256SUMS(repo, tag, sumsPath); err != nil {
			return fmt.Errorf("%s@%s: %w", repo, tag, err)
		}
		if err := ghrelease.VerifySHA256SUMS(sumsPath, dest); err != nil {
			return fmt.Errorf("%s@%s asset verify: %w", repo, tag, err)
		}
		if err := extractBinary(dest, bin, binName); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no usable asset for %s@%s", repo, tag)
}

func extractBinary(tarball, binDir, wantName string) error {
	f, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	var fallback string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		if base == wantName {
			return writeExecutable(tr, filepath.Join(binDir, wantName))
		}
		switch base {
		case "LICENSE", "LICENSE.txt", "README", "README.md":
			continue
		}
		if fallback == "" {
			fallback = hdr.Name
		}
	}
	if fallback == "" {
		return fmt.Errorf("tarball %s had no usable binary", tarball)
	}
	f2, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer func() { _ = f2.Close() }()
	gz2, err := gzip.NewReader(f2)
	if err != nil {
		return err
	}
	defer func() { _ = gz2.Close() }()
	tr2 := tar.NewReader(gz2)
	for {
		hdr, err := tr2.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Name == fallback {
			return writeExecutable(tr2, filepath.Join(binDir, wantName))
		}
	}
	return fmt.Errorf("tarball %s had no usable binary", tarball)
}

func writeExecutable(r io.Reader, dest string) error {
	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

func copyFromLab(dirs []string, bin, name string) bool {
	for _, d := range dirs {
		if d == "" {
			continue
		}
		src := filepath.Join(d, name)
		if isExecutable(src) {
			data, err := os.ReadFile(src)
			if err != nil {
				continue
			}
			if err := os.WriteFile(filepath.Join(bin, name), data, 0o755); err == nil {
				return true
			}
		}
	}
	return false
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func isExecutable(p string) bool {
	info, err := os.Stat(p)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

// LabBinDirs mirrors resolve_lab_bin from lib/common.sh.
func LabBinDirs(root string) []string {
	var dirs []string
	if v := os.Getenv("MUXCORE_LAB_BIN"); v != "" {
		dirs = append(dirs, v)
	}
	home, _ := os.UserHomeDir()
	dirs = append(dirs,
		filepath.Join(root, "..", "_mvp", "bin"),
		filepath.Join(root, "..", "..", "_mvp", "bin"),
		filepath.Join(home, "Projects", "MuxCore", "_mvp", "bin"),
	)
	return dirs
}
