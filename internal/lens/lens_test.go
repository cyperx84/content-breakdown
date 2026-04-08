package lens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cyperx84/content-breakdown/internal/schema"
)

func TestLoadLensAllBuiltIn(t *testing.T) {
	repoRoot := findRepoRoot(t)
	lensesDir := filepath.Join(repoRoot, "lenses")

	entries, err := os.ReadDir(lensesDir)
	if err != nil {
		t.Fatalf("read lenses dir: %v", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			path := filepath.Join(lensesDir, entry.Name())
			lens, err := LoadLens(path)
			if err != nil {
				t.Fatalf("LoadLens failed: %v", err)
			}
			if lens.ID == "" {
				t.Error("empty ID")
			}
			if lens.Name == "" {
				t.Error("empty Name")
			}
			if lens.Purpose == "" {
				t.Error("empty Purpose")
			}
			if len(lens.Questions) == 0 {
				t.Error("no Questions")
			}
			if len(lens.RankingDimensions) == 0 {
				t.Error("no RankingDimensions")
			}
		})
	}
}

func TestLoadLensErrors(t *testing.T) {
	t.Run("nonexistent file", func(t *testing.T) {
		_, err := LoadLens("/nonexistent/path/lens.json")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "bad.json")
		if err := os.WriteFile(path, []byte("not valid json {"), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadLens(path)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "empty.json")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadLens(path)
		if err == nil {
			t.Fatal("expected error for empty file")
		}
	})
}

func TestParseResponseEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "clean JSON",
			input:   `{"relevanceScore": 0.8, "rationale": "test", "rankedIdeas": []}`,
			wantErr: false,
		},
		{
			name:    "JSON in code block",
			input:   "```json\n{\"relevanceScore\": 0.7, \"rationale\": \"test\", \"rankedIdeas\": []}\n```",
			wantErr: false,
		},
		{
			name:    "JSON with surrounding text",
			input:   "Here's the analysis:\n\n{\"relevanceScore\": 0.6, \"rationale\": \"test\", \"rankedIdeas\": []}\n\nDone.",
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   "not json",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "missing required fields",
			input:   `{}`,
			wantErr: false,
		},
		{
			name:    "negative relevance",
			input:   `{"relevanceScore": -0.5, "rationale": "test", "rankedIdeas": []}`,
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseResponse(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("nil result without error")
			}
		})
	}
}

func TestBuildPromptIncludesAllFields(t *testing.T) {
	lensDef := &LensDefinition{
		ID:                  "test-lens",
		Name:                "Test Lens",
		Purpose:             "Test purpose",
		Questions:           []string{"Q1?", "Q2?"},
		RankingDimensions:   []string{"dim1", "dim2"},
		IgnoreRules:         []string{"ignore X", "ignore Y"},
		ProjectContextHints: []string{"hint-one", "hint-two"},
		ArtifactRules: map[string][]string{
			"high": {"write PRD"},
		},
	}

	src := &schema.SourceRecord{
		ID:           "test-id",
		CanonicalURL: "https://example.com/test",
		Title:        "Test Title",
		Author:       "Test Author",
	}
	ext := &schema.ExtractionRecord{
		Summary:       "Test summary text.",
		Tools:         []string{"Tool1", "Tool2"},
		Workflows:     []string{"Workflow1"},
		Opportunities: []string{"Opp1"},
		Claims:        []string{"Claim1"},
		Quotes:        []string{"Quote1"},
	}

	prompt, err := buildPrompt(src, ext, lensDef)
	if err != nil {
		t.Fatalf("buildPrompt error: %v", err)
	}

	mustContain := []string{
		lensDef.Name,
		lensDef.Purpose,
		lensDef.Questions[0],
		lensDef.RankingDimensions[0],
		lensDef.IgnoreRules[0],
		lensDef.ProjectContextHints[0],
		"write PRD",
		src.Title,
		src.Author,
		ext.Summary,
		ext.Tools[0],
		ext.Workflows[0],
		ext.Opportunities[0],
		ext.Claims[0],
		ext.Quotes[0],
		"relevanceScore",
		"rankedIdeas",
	}

	for _, want := range mustContain {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
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
			t.Fatal("repo root not found")
		}
		dir = parent
	}
}
