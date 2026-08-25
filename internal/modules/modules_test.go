package modules

import (
	"slices"
	"testing"
)

func TestResolveNeverIncludesAcquisitionModules(t *testing.T) {
	out := Resolve(Answers{
		Libraries: []string{"Movies", "TV"},
		Playback:  []string{"MuxCore player"},
		Profile:   "sqlite",
	})
	for name := range AcquisitionNever {
		if slices.Contains(out, name) {
			t.Errorf("Resolve() must never include acquisition module %q, got %v", name, out)
		}
	}
	if !slices.Contains(out, "database-sqlite") {
		t.Errorf("expected database-sqlite in %v", out)
	}
	if slices.Contains(out, "database-postgres") {
		t.Errorf("did not expect database-postgres in %v", out)
	}
	for _, want := range []string{"media-movies", "media-tvshows", "media-scanner", "metadata-tmdb", "media-transcoder"} {
		if !slices.Contains(out, want) {
			t.Errorf("expected %q in %v", want, out)
		}
	}
}

func TestResolvePostgresProfile(t *testing.T) {
	out := Resolve(Answers{Profile: "postgres"})
	if !slices.Contains(out, "database-postgres") {
		t.Errorf("expected database-postgres in %v", out)
	}
	if slices.Contains(out, "database-sqlite") {
		t.Errorf("did not expect database-sqlite in %v", out)
	}
}

func TestResolveNoLibrariesSkipsVideoHelpers(t *testing.T) {
	out := Resolve(Answers{Profile: "sqlite"})
	for _, unwanted := range []string{"media-scanner", "media-root-folders", "metadata-tmdb", "media-movies", "media-tvshows"} {
		if slices.Contains(out, unwanted) {
			t.Errorf("did not expect %q with no libraries selected, got %v", unwanted, out)
		}
	}
}
