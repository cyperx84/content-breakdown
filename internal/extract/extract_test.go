package extract

import (
	"strings"
	"testing"
)

func TestParseResponseJSONEdgeCases(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantOK  bool
		summary string
	}{
		{
			name:    "clean JSON",
			input:   `{"summary": "test summary", "tools": ["a", "b"]}`,
			wantOK:  true,
			summary: "test summary",
		},
		{
			name:    "JSON in code block",
			input:   "```\n{\"summary\": \"block\", \"tools\": []}\n```",
			wantOK:  true,
			summary: "block",
		},
		{
			name:    "JSON in fenced block with language",
			input:   "```json\n{\"summary\": \"fenced\", \"tools\": []}\n```",
			wantOK:  true,
			summary: "fenced",
		},
		{
			name:    "JSON with leading text",
			input:   "Here is the result:\n\n{\"summary\": \"lead\", \"tools\": []}",
			wantOK:  true,
			summary: "lead",
		},
		{
			name:    "JSON with trailing text",
			input:   "{\"summary\": \"trail\", \"tools\": []}\n\nLet me know if you need more.",
			wantOK:  true,
			summary: "trail",
		},
		{
			name:   "invalid JSON",
			input:  "not json at all",
			wantOK: false,
		},
		{
			name:   "empty input",
			input:  "",
			wantOK: false,
		},
		{
			name:   "just code fence",
			input:  "```\n```",
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseResponse(tc.input)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if strings.TrimSpace(got.Summary) != tc.summary {
					t.Fatalf("summary = %q, want %q", got.Summary, tc.summary)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
			}
		})
	}
}

func TestChunkTranscriptEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxChars  int
		wantCount int
	}{
		{
			name:      "empty string",
			input:     "",
			maxChars:  1000,
			wantCount: 0,
		},
		{
			name:      "short text",
			input:     "hello world",
			maxChars:  1000,
			wantCount: 1,
		},
		{
			name:      "exact boundary",
			input:     strings.Repeat("word ", 20),
			maxChars:  100,
			wantCount: 1,
		},
		{
			name:      "needs split",
			input:     strings.Repeat("word ", 200),
			maxChars:  50,
			wantCount: 4, // 200 words * 5 chars each = 1000 chars, / 50 = 20 chunks, but word-boundary aware
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			chunks := chunkTranscript(tc.input, tc.maxChars)
			if tc.wantCount == 0 {
				if len(chunks) != 0 {
					t.Fatalf("expected 0 chunks, got %d", len(chunks))
				}
				return
			}
			if len(chunks) == 0 {
				t.Fatal("expected at least 1 chunk, got 0")
			}
			// Verify no chunk exceeds maxChars
			for i, c := range chunks {
				if len(c) > tc.maxChars {
					t.Fatalf("chunk %d exceeds maxChars: %d > %d", i, len(c), tc.maxChars)
				}
			}
		})
	}
}

func TestMergeOutputsDeduplicationAndSorting(t *testing.T) {
	outputs := []*ExtractionOutput{
		{
			Summary:   "First summary.",
			Tools:     []string{"Claude", "Codex", "GPT"},
			Workflows: []string{"Plan first", "Test often"},
		},
		{
			Summary:   "Second summary.",
			Tools:     []string{"claude", "Gemini", "CODEX"}, // duplicates with different case
			Workflows: []string{"PLAN FIRST", "Ship fast"},
		},
		{
			Summary:   "Third summary.",
			Tools:     []string{"Claude", "Gemini"}, // more duplicates
			Workflows: []string{"plan first"},       // duplicate again
		},
	}

	merged := mergeOutputs(outputs)

	// Should contain all three summaries
	if !strings.Contains(merged.Summary, "First summary.") {
		t.Error("missing first summary")
	}
	if !strings.Contains(merged.Summary, "Second summary.") {
		t.Error("missing second summary")
	}
	if !strings.Contains(merged.Summary, "Third summary.") {
		t.Error("missing third summary")
	}

	// Should have deduplicated tools (case-insensitive)
	// Claude, Codex, GPT, Gemini = 4 unique
	if len(merged.Tools) != 4 {
		t.Fatalf("expected 4 unique tools, got %d: %v", len(merged.Tools), merged.Tools)
	}

	// Should be sorted alphabetically
	for i := 1; i < len(merged.Tools); i++ {
		if merged.Tools[i-1] > merged.Tools[i] {
			t.Fatalf("tools not sorted: %v", merged.Tools)
		}
	}

	// Should have deduplicated workflows (case-insensitive)
	// "Plan first", "Test often", "Ship fast" = 3 unique
	if len(merged.Workflows) != 3 {
		t.Fatalf("expected 3 unique workflows, got %d: %v", len(merged.Workflows), merged.Workflows)
	}
}

func TestUniqueStringsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		wantLen  int
		wantSort bool
	}{
		{
			name:     "nil slice",
			input:    nil,
			wantLen:  0,
			wantSort: true,
		},
		{
			name:     "empty slice",
			input:    []string{},
			wantLen:  0,
			wantSort: true,
		},
		{
			name:     "all duplicates",
			input:    []string{"a", "A", "a ", " A "},
			wantLen:  1,
			wantSort: true,
		},
		{
			name:     "mixed with empty strings",
			input:    []string{"a", "", "b", "  ", "c"},
			wantLen:  3,
			wantSort: true,
		},
		{
			name:     "already sorted",
			input:    []string{"a", "b", "c"},
			wantLen:  3,
			wantSort: true,
		},
		{
			name:     "reverse sorted",
			input:    []string{"c", "b", "a"},
			wantLen:  3,
			wantSort: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := uniqueStrings(tc.input)
			if len(got) != tc.wantLen {
				t.Fatalf("expected %d items, got %d: %v", tc.wantLen, len(got), got)
			}
			if tc.wantSort && len(got) > 1 {
				for i := 1; i < len(got); i++ {
					if got[i-1] > got[i] {
						t.Fatalf("result not sorted: %v", got)
					}
				}
			}
		})
	}
}

func TestExtractJSONObject(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOK   bool
		contains string
	}{
		{
			name:     "simple object",
			input:    `{"a": 1}`,
			wantOK:   true,
			contains: `"a": 1`,
		},
		{
			name:     "nested object",
			input:    `{"outer": {"inner": 2}}`,
			wantOK:   true,
			contains: `"inner": 2`,
		},
		{
			name:     "with whitespace",
			input:    "  \n  {  \"key\"  :  \"value\"  }  \n  ",
			wantOK:   true,
			contains: `"key"`,
		},
		{
			name:   "no object",
			input:  "no json here at all",
			wantOK: false,
		},
		{
			name:   "just brace",
			input:  "{",
			wantOK: false,
		},
		{
			name:   "reversed braces",
			input:  "}{",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractJSONObject(tc.input)
			if tc.wantOK {
				if got == "" {
					t.Fatal("expected JSON object, got empty string")
				}
				if !strings.Contains(got, tc.contains) {
					t.Fatalf("result %q doesn't contain %q", got, tc.contains)
				}
			} else {
				if got != "" {
					t.Fatalf("expected empty string, got %q", got)
				}
			}
		})
	}
}
