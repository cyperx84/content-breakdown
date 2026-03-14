package emit

import (
	"strings"
	"testing"
	"time"

	"github.com/cyperx84/content-breakdown/internal/schema"
)

func TestVaultNoteIncludesQuotedFrontmatterAndMetadata(t *testing.T) {
	now := time.Date(2026, 3, 14, 10, 30, 0, 0, time.UTC)
	published := "2026-03-01"
	duration := "12m5s"
	note := VaultNote(
		&schema.SourceRecord{
			ID:           "yt_123",
			CanonicalURL: "https://youtube.com/watch?v=123",
			Title:        `Builder: "Ship it"`,
			Author:       "CyperX",
			PublishedAt:  &published,
			Duration:     &duration,
			Metadata:     schema.SourceMetadata{ExtractedAt: now},
		},
		&schema.ExtractionRecord{Summary: "Summary", Opportunities: []string{"Do X"}},
		&schema.LensResult{LensID: "openclaw-product", RelevanceScore: 0.8, Rationale: "Useful", RankedIdeas: []schema.RankedIdea{{Title: "Idea", Score: 0.9, Rationale: "Because"}}},
	)

	if !strings.Contains(note, `title: "Builder: \"Ship it\" Breakdown"`) {
		t.Fatalf("expected quoted title in frontmatter:\n%s", note)
	}
	if !strings.Contains(note, "- **Published:** 2026-03-01") {
		t.Fatalf("expected published metadata:\n%s", note)
	}
	if !strings.Contains(note, "- **Duration:** 12m5s") {
		t.Fatalf("expected duration metadata:\n%s", note)
	}
	if !strings.Contains(note, "**Rationale:** Because") {
		t.Fatalf("expected idea rationale:\n%s", note)
	}
}
