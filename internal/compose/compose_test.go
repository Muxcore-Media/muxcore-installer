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

func writeAndRead(t *testing.T, env Env) string {
	t.Helper()
	if _, err := Write(env); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(env.Root, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestWriteHouseholdByDefault(t *testing.T) {
	root := t.TempDir()
	s := writeAndRead(t, Env{Root: root, EnabledModules: []string{"muxcored", "admin-ui", "api-rest"}, EnableMediaUI: true})
	for _, bad := range []string{"MUXCORE_INSECURE_DISABLE_TLS", "ADMIN_UI_INSECURE", "MUXCORE_PROFILE: dev"} {
		if strings.Contains(s, bad) {
			t.Errorf("household compose contains %s:\n%s", bad, s)
		}
	}
	for _, want := range []string{
		"MUXCORE_PROFILE: household",
		"MUXCORE_ENROLL_SECRET: ${MUXCORE_ENROLL_SECRET:?",
		"MUXCORE_GRPC_CA_CERT_DIR: /app/ca",
		"MUXCORE_CA_EXPORT_DIR: /app/mesh-ca",
		root + "/mesh/ca:/app/ca",
		root + "/mesh/public:/app/mesh-ca",
		"--cacert /app/mesh-ca/ca.crt https://127.0.0.1:8080/health",
		"MUXCORE_BOOTSTRAP_TOKEN: ${MUXCORE_ENROLL_TOKEN_API_REST:?",
		"MUXCORE_BOOTSTRAP_TOKEN: ${MUXCORE_ENROLL_TOKEN_ADMIN_UI:?",
		"MUXCORE_BOOTSTRAP_TOKEN: ${MUXCORE_ENROLL_TOKEN_MEDIA_UI:?",
		"MUXCORE_MODULE_ID: media-ui",
		root + "/mesh/id/api-rest:/mesh/id",
		root + "/mesh/id/media-ui:/mesh/id",
		root + "/mesh/public:/mesh/ca:ro",
		"MUXCORE_TLS_CA: /mesh/ca/ca.crt",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("household compose lacks %q:\n%s", want, s)
		}
	}
	// The CA key dir is never inside the data dir every module mounts.
	if strings.Contains(s, root+"/data/ca") {
		t.Error("CA under data/")
	}
}

func TestWriteDevProfile(t *testing.T) {
	root := t.TempDir()
	s := writeAndRead(t, Env{Root: root, Profile: "dev", EnabledModules: []string{"admin-ui"}})
	for _, want := range []string{"MUXCORE_PROFILE: dev", `MUXCORE_INSECURE_DISABLE_TLS: "true"`, `ADMIN_UI_INSECURE: "true"`, "curl -sf http://127.0.0.1:8080/health"} {
		if !strings.Contains(s, want) {
			t.Errorf("dev compose lacks %q", want)
		}
	}
	for _, bad := range []string{"MUXCORE_ENROLL_SECRET", "MUXCORE_BOOTSTRAP_TOKEN", "/mesh/"} {
		if strings.Contains(s, bad) {
			t.Errorf("dev compose contains %s", bad)
		}
	}
}

func TestModuleIDs(t *testing.T) {
	got := ModuleIDs([]string{"muxcored", "api-rest", "mediauiprox"}, true)
	if strings.Join(got, ",") != "api-rest,media-ui" {
		t.Errorf("ModuleIDs = %v", got)
	}
}
