package ghrelease

import (
	"os"
	"testing"
)

func TestGitHubAssetURL(t *testing.T) {
	t.Setenv("MUXCORE_GITHUB_ORG", "Muxcore-Media")
	got := GitHubAssetURL("auth-local", "v0.1.5", "auth-local_0.1.5_linux_amd64.tar.gz")
	want := "https://github.com/Muxcore-Media/auth-local/releases/download/v0.1.5/auth-local_0.1.5_linux_amd64.tar.gz"
	if got != want {
		t.Fatalf("GitHubAssetURL = %q, want %q", got, want)
	}
}

func TestAssetURLIsGitHub(t *testing.T) {
	t.Setenv("MUXCORE_GITHUB_ORG", "")
	got := AssetURL("muxcore-installer", "v0.3.3", "SHA256SUMS")
	want := "https://github.com/Muxcore-Media/muxcore-installer/releases/download/v0.3.3/SHA256SUMS"
	if got != want {
		t.Fatalf("AssetURL = %q, want %q", got, want)
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
