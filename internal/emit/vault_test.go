package emit

import (
	"strings"
	"testing"
	"time"

	"github.com/cyperx84/content-breakdown/internal/schema"
)

func TestVaultNoteIncludesFrontmatterAndMetadata(t *testing.T) {
	now := time.Date(2026, 3, 14, 10, 30, 0, 0, time.UTC)
	note := VaultNote(
		&schema.SourceRecord{
			ID:           "yt_123",
			CanonicalURL: "https://youtube.com/watch?v=123",
			Title:        `Builder: "Ship it"`,
			Author:       "CyperX",
			PublishedAt:  "2026-03-01",
			Duration:     "12m5s",
			Metadata:     schema.SourceMetadata{ExtractedAt: now},
		},
		&schema.ExtractionRecord{Summary: "Summary", Opportunities: []string{"Do X"}},
		&schema.LensResult{LensID: "openclaw-product", LensName: "OpenClaw Product Lens", RelevanceScore: 0.8, Rationale: "Useful", RankedIdeas: []schema.RankedIdea{{Title: "Idea", Score: 0.9, Rationale: "Because"}}},
	)

	for _, want := range []string{
		`title: "Builder: \"Ship it\" Breakdown"`,
		`author: "CyperX"`,
		`published: 2026-03-01`,
		`duration: "12m5s"`,
		"- **Published:** 2026-03-01",
		"- **Duration:** 12m5s",
		"**Rationale:** Because",
		"What Matters for OpenClaw Product Lens",
	} {
		if !strings.Contains(note, want) {
			t.Errorf("note missing %q\n---\n%s", want, note)
		}
	}
}

func TestVaultNoteFallsBackToLensIDWhenNameMissing(t *testing.T) {
	note := VaultNote(
		&schema.SourceRecord{Title: "X", Metadata: schema.SourceMetadata{ExtractedAt: time.Now()}},
		&schema.ExtractionRecord{Summary: "x"},
		&schema.LensResult{LensID: "personal-os", Rationale: "x"},
	)
	if !strings.Contains(note, "What Matters for personal-os") {
		t.Errorf("expected fallback to lens ID, got:\n%s", note)
	}
}
