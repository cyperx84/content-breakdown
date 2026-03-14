package extract

import (
	"strings"
	"testing"
)

func TestExtractJSONObject(t *testing.T) {
	input := "here you go\n```json\n{\n  \"summary\": \"ok\",\n  \"tools\": []\n}\n```"
	got := extractJSONObject(input)
	if !strings.Contains(got, `"summary": "ok"`) {
		t.Fatalf("extractJSONObject() = %q", got)
	}
}

func TestChunkTranscript(t *testing.T) {
	input := strings.Repeat("word ", 100)
	chunks := chunkTranscript(input, 50)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > 50 {
			t.Fatalf("chunk %d too large: %d", i, len(c))
		}
	}
}

func TestMergeOutputsDedupes(t *testing.T) {
	merged := mergeOutputs([]*ExtractionOutput{
		{Summary: "one", Tools: []string{"Go", "Cobra"}, Workflows: []string{"Ship"}},
		{Summary: "two", Tools: []string{"go", "yt-dlp"}, Workflows: []string{"Ship", "Test"}},
	})
	if len(merged.Tools) != 3 {
		t.Fatalf("expected 3 deduped tools, got %d: %#v", len(merged.Tools), merged.Tools)
	}
	if !strings.Contains(merged.Summary, "one") || !strings.Contains(merged.Summary, "two") {
		t.Fatalf("merged summary missing content: %q", merged.Summary)
	}
}
