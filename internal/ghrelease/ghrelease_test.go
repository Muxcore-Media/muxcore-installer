package ghrelease

import (
	"os"
	"testing"
)

func TestForgejoAssetURL(t *testing.T) {
	t.Setenv("MUXCORE_FORGEJO_URL", "https://git.example.test")
	t.Setenv("MUXCORE_FORGEJO_ORG", "muxcore")
	got := ForgejoAssetURL("core", "v0.5.0", "muxcored_0.5.0_linux_amd64.tar.gz")
	want := "https://git.example.test/muxcore/core/releases/download/v0.5.0/muxcored_0.5.0_linux_amd64.tar.gz"
	if got != want {
		t.Fatalf("ForgejoAssetURL = %q, want %q", got, want)
	}
}

func TestGitHubAssetURL(t *testing.T) {
	t.Setenv("MUXCORE_GITHUB_ORG", "Muxcore-Media")
	got := GitHubAssetURL("auth-local", "v0.1.5", "auth-local_0.1.5_linux_amd64.tar.gz")
	want := "https://github.com/Muxcore-Media/auth-local/releases/download/v0.1.5/auth-local_0.1.5_linux_amd64.tar.gz"
	if got != want {
		t.Fatalf("GitHubAssetURL = %q, want %q", got, want)
	}
}

func TestAssetURLPrefersForgejo(t *testing.T) {
	t.Setenv("MUXCORE_FORGEJO_URL", "https://git.zem.systems")
	got := AssetURL("muxcore-installer", "v0.3.3", "SHA256SUMS")
	if got != ForgejoAssetURL("muxcore-installer", "v0.3.3", "SHA256SUMS") {
		t.Fatalf("AssetURL should match Forgejo URL, got %q", got)
	}
}

func TestVerifySHA256SUMS(t *testing.T) {
	dir := t.TempDir()
	sums := dir + "/SHA256SUMS"
	file := dir + "/artifact.tar.gz"
	if err := os.WriteFile(file, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256SUMS(sums, file); err == nil {
		t.Fatal("expected error when SHA256SUMS missing")
	}
	if err := os.WriteFile(sums, []byte("deadbeef  artifact.tar.gz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256SUMS(sums, file); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}
