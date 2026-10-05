package compose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteRegistryPrefix(t *testing.T) {
	t.Setenv("MUXCORE_REGISTRY", "registry.example/muxcore")
	root := t.TempDir()
	_, err := Write(Env{
		Root:           root,
		EnabledModules: []string{"admin-ui"},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "image: registry.example/muxcore/muxcored:") {
		t.Errorf("core image should use MUXCORE_REGISTRY, got:\n%s", s)
	}
	if !strings.Contains(s, "image: registry.example/muxcore/admin-ui:") {
		t.Errorf("admin-ui image should use MUXCORE_REGISTRY")
	}
}

func TestWriteLibraryMounts(t *testing.T) {
	root := t.TempDir()
	_, err := Write(Env{
		Root:           root,
		EnabledModules: []string{"media-scanner"},
		LibraryRoot:    "/host/movies",
		TVLibraryRoot:  "/host/tv",
		ImportDir:      "/host/import",
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"/host/movies:/data/library",
		"/host/tv:/data/library/tv",
		"/host/import:/data/import",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("compose missing mount %q in:\n%s", want, s)
		}
	}
}

func TestWriteMediauiproxWhenPlayerSelected(t *testing.T) {
	root := t.TempDir()
	dist := filepath.Join(root, "dist-app")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Write(Env{
		Root:           root,
		EnabledModules: []string{"media-transcoder"},
		EnableMediaUI:  true,
		MediaUIDist:    dist,
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "mediauiprox:") {
		t.Fatalf("expected mediauiprox service, got:\n%s", s)
	}
	if !strings.Contains(s, "5173:5173") {
		t.Errorf("expected player port mapping")
	}
	if !strings.Contains(s, dist+":/dist-app") {
		t.Errorf("expected dist-app volume mount for %s", dist)
	}
}

func TestRegistryPrefixDefault(t *testing.T) {
	t.Setenv("MUXCORE_REGISTRY", "")
	if got := RegistryPrefix(); got != "localhost:5000/muxcore" {
		t.Fatalf("default registry = %q", got)
	}
}
