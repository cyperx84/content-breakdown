package lens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLens(t *testing.T) {
	// Find lenses directory relative to test
	root := findRepoRoot(t)
	lensDir := filepath.Join(root, "lenses")

	entries, err := os.ReadDir(lensDir)
	if err != nil {
		t.Fatalf("read lenses dir: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("no lens files found")
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(lensDir, entry.Name())
		lens, err := LoadLens(path)
		if err != nil {
			t.Fatalf("LoadLens(%s): %v", entry.Name(), err)
		}
		if lens.ID == "" {
			t.Fatalf("lens %s: empty ID", entry.Name())
		}
		if lens.Name == "" {
			t.Fatalf("lens %s: empty Name", entry.Name())
		}
		if lens.Purpose == "" {
			t.Fatalf("lens %s: empty Purpose", entry.Name())
		}
		if len(lens.Questions) == 0 {
			t.Fatalf("lens %s: no Questions", entry.Name())
		}
		if len(lens.RankingDimensions) == 0 {
			t.Fatalf("lens %s: no RankingDimensions", entry.Name())
		}
	}
}

func TestExtractJSONObject(t *testing.T) {
	cases := map[string]bool{
		`{"relevanceScore": 0.5}`:                       true,
		"here is your result:\n```json\n{\"a\":1}\n```": true,
		"no json here":                                  false,
		"":                                              false,
	}
	for input, wantOK := range cases {
		got := extractJSONObject(input)
		if wantOK && got == "" {
			t.Fatalf("extractJSONObject(%q) = empty, want JSON", input)
		}
		if !wantOK && got != "" {
			t.Fatalf("extractJSONObject(%q) = %q, want empty", input, got)
		}
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}
