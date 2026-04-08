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
		name     string
		input    string
		maxChars int
		wantZero bool
	}{
		{name: "empty string", input: "", maxChars: 1000, wantZero: true},
		{name: "short text", input: "hello world", maxChars: 1000},
		{name: "exact boundary", input: strings.Repeat("word ", 20), maxChars: 100},
		{name: "needs split", input: strings.Repeat("word ", 200), maxChars: 50},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			chunks := chunkTranscript(tc.input, tc.maxChars)
			if tc.wantZero {
				if len(chunks) != 0 {
					t.Fatalf("expected 0 chunks, got %d", len(chunks))
				}
				return
			}
			if len(chunks) == 0 {
				t.Fatal("expected at least 1 chunk, got 0")
			}
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
			Tools:     []string{"claude", "Gemini", "CODEX"},
			Workflows: []string{"PLAN FIRST", "Ship fast"},
		},
		{
			Summary:   "Third summary.",
			Tools:     []string{"Claude", "Gemini"},
			Workflows: []string{"plan first"},
		},
	}

	merged := mergeOutputs(outputs)

	if !strings.Contains(merged.Summary, "First summary.") {
		t.Error("missing first summary")
	}
	if !strings.Contains(merged.Summary, "Second summary.") {
		t.Error("missing second summary")
	}
	if !strings.Contains(merged.Summary, "Third summary.") {
		t.Error("missing third summary")
	}

	// Claude, Codex, GPT, Gemini = 4 unique
	if len(merged.Tools) != 4 {
		t.Fatalf("expected 4 unique tools, got %d: %v", len(merged.Tools), merged.Tools)
	}

	for i := 1; i < len(merged.Tools); i++ {
		if merged.Tools[i-1] > merged.Tools[i] {
			t.Fatalf("tools not sorted: %v", merged.Tools)
		}
	}

	// "Plan first", "Test often", "Ship fast" = 3 unique
	if len(merged.Workflows) != 3 {
		t.Fatalf("expected 3 unique workflows, got %d: %v", len(merged.Workflows), merged.Workflows)
	}
}
