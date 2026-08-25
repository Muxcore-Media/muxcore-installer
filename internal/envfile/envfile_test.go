package envfile

import (
	"path/filepath"
	"testing"
)

// TestSaveQuotesMultiWordValues locks in a real bug: up.sh and
// bootstrap-auth.sh `source` this file, so an unquoted value containing
// spaces (e.g. ENABLED_MODULES, a space-separated module list) was parsed
// as a command invocation instead of a variable assignment.
func TestSaveQuotesMultiWordValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	f, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	f.Set("ENABLED_MODULES", "muxcored api-rest auth-local")
	f.Set("MVP_ADMIN_PASSWORD", `p@ss"word\with$special`+"`chars`")
	if err := f.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := reloaded.Get("ENABLED_MODULES", ""); got != "muxcored api-rest auth-local" {
		t.Errorf("ENABLED_MODULES round-trip = %q", got)
	}
	if got := reloaded.Get("MVP_ADMIN_PASSWORD", ""); got != `p@ss"word\with$special`+"`chars`" {
		t.Errorf("MVP_ADMIN_PASSWORD round-trip = %q", got)
	}
}

func TestSetAllDeterministicOrder(t *testing.T) {
	dir := t.TempDir()
	f, err := Load(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	f.SetAll(map[string]string{"ZEBRA": "1", "APPLE": "2", "MANGO": "3"})
	if got := []string{"APPLE", "MANGO", "ZEBRA"}; !equalOrder(f.order, got) {
		t.Errorf("order = %v, want sorted %v", f.order, got)
	}
}

func equalOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
