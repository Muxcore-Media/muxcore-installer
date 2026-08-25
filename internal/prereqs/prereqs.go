// Package prereqs detects the host OS family and builds the package-manager
// commands needed for optional dependencies (ffmpeg, Docker). Actual
// installs are executed by the caller via internal/execstream so their
// output can stream into the TUI's viewport. Ported from lib/prereqs.sh.
package prereqs

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
	"time"
)

// Family is a coarse OS/package-manager bucket.
type Family string

const (
	Debian  Family = "debian"
	RHEL    Family = "rhel"
	Arch    Family = "arch"
	Alpine  Family = "alpine"
	NixOS   Family = "nixos"
	Darwin  Family = "darwin"
	Unknown Family = "unknown"
)

// DetectFamily mirrors prereqs_os_family from lib/prereqs.sh.
func DetectFamily() Family {
	if runtime.GOOS == "darwin" {
		return Darwin
	}
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return Unknown
	}
	defer f.Close()
	kv := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		kv[line[:eq]] = strings.Trim(line[eq+1:], `"`)
	}
	switch kv["ID"] {
	case "debian", "ubuntu", "raspbian", "linuxmint", "pop":
		return Debian
	case "fedora", "rhel", "centos", "rocky", "almalinux":
		return RHEL
	case "arch", "manjaro", "endeavouros":
		return Arch
	case "alpine":
		return Alpine
	case "nixos":
		return NixOS
	}
	idLike := kv["ID_LIKE"]
	switch {
	case strings.Contains(idLike, "debian"):
		return Debian
	case strings.Contains(idLike, "rhel"), strings.Contains(idLike, "fedora"):
		return RHEL
	}
	return Unknown
}

// InstallArgv returns the argv (without sudo) to install pkgs on this
// family, or nil if the family needs manual/nix-shell instructions instead.
func InstallArgv(f Family, pkgs []string) []string {
	switch f {
	case Debian:
		return append([]string{"apt-get", "install", "-y"}, pkgs...)
	case RHEL:
		if _, err := exec.LookPath("dnf"); err == nil {
			return append([]string{"dnf", "install", "-y"}, pkgs...)
		}
		return append([]string{"yum", "install", "-y"}, pkgs...)
	case Arch:
		return append([]string{"pacman", "-Sy", "--noconfirm"}, pkgs...)
	case Alpine:
		return append([]string{"apk", "add", "--no-cache"}, pkgs...)
	case Darwin:
		return append([]string{"brew", "install"}, pkgs...)
	default:
		return nil
	}
}

// NeedsSudo reports whether InstallArgv's command must run under sudo.
func NeedsSudo(f Family) bool {
	return f == Debian || f == RHEL || f == Arch || f == Alpine
}

// IsRoot reports whether the current process is running as root/uid 0.
func IsRoot() bool {
	u, err := user.Current()
	return err == nil && u.Uid == "0"
}

// HaveCmd is a small command-exists helper used across the wizard.
func HaveCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// HaveFFmpeg reports whether both ffmpeg and ffprobe are on PATH.
func HaveFFmpeg() bool {
	return HaveCmd("ffmpeg") && HaveCmd("ffprobe")
}

// HaveCompose reports whether some flavor of Docker/Podman Compose works.
func HaveCompose() bool {
	if HaveCmd("docker") {
		if err := exec.Command("docker", "compose", "version").Run(); err == nil {
			return true
		}
	}
	if HaveCmd("podman") {
		if err := exec.Command("podman", "compose", "version").Run(); err == nil {
			return true
		}
	}
	return HaveCmd("docker-compose") || HaveCmd("podman-compose")
}

// HaveDocker reports a usable, running Docker daemon (or Podman as docker).
func HaveDocker() bool {
	if !HaveCmd("docker") {
		return false
	}
	return exec.Command("docker", "info").Run() == nil
}

// HWAccel describes a detected hardware video acceleration path.
type HWAccel struct {
	Kind string // vaapi | nvenc | videotoolbox
	Note string
}

// DetectHWAccel does a best-effort, non-fatal probe for GPU transcode paths.
// It never fails the wizard — absence just means software-only transcoding.
func DetectHWAccel() []HWAccel {
	var found []HWAccel
	switch runtime.GOOS {
	case "linux":
		if entries, err := os.ReadDir("/dev/dri"); err == nil {
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), "renderD") {
					found = append(found, HWAccel{Kind: "vaapi", Note: "/dev/dri/" + e.Name()})
					break
				}
			}
		}
		if HaveCmd("nvidia-smi") {
			found = append(found, HWAccel{Kind: "nvenc", Note: "nvidia-smi present"})
		}
	case "darwin":
		found = append(found, HWAccel{Kind: "videotoolbox", Note: "Apple VideoToolbox"})
	}
	return found
}

// PortInUse does a quick local TCP dial to see whether something is already
// listening on the given port (used for the pre-flight conflict check).
func PortInUse(port string) bool {
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 200*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}
