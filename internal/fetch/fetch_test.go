package fetch

import (
	"testing"
)

func TestLabBinDirsIncludesOverride(t *testing.T) {
	t.Setenv("MUXCORE_LAB_BIN", "/tmp/lab-bin")
	dirs := LabBinDirs("/install")
	if dirs[0] != "/tmp/lab-bin" {
		t.Fatalf("LabBinDirs[0] = %q, want override first", dirs[0])
	}
}

func TestOSArchSupported(t *testing.T) {
	osName, arch, err := OSArch()
	if err != nil {
		t.Fatal(err)
	}
	if osName == "" || arch == "" {
		t.Fatalf("os/arch = %q %q", osName, arch)
	}
}
