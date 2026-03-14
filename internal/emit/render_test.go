package emit

import (
	"strings"
	"testing"
	"time"

	"github.com/cyperx84/content-breakdown/internal/schema"
)

func sampleRecords() (*schema.SourceRecord, *schema.ExtractionRecord, *schema.LensResult) {
	published := "2026-03-01"
	duration := "10m0s"
	src := &schema.SourceRecord{
		ID:           "src1",
		CanonicalURL: "https://example.com/video",
		Title:        "Agent Patterns",
		Author:       "Chris",
		PublishedAt:  &published,
		Duration:     &duration,
		Metadata:     schema.SourceMetadata{ExtractedAt: time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)},
	}
	ext := &schema.ExtractionRecord{
		Summary:       "A strong talk about agent orchestration.",
		Tools:         []string{"OpenClaw", "yt-dlp"},
		Workflows:     []string{"Extract -> Lens -> Emit"},
		Opportunities: []string{"Build better emitters"},
		Claims:        []string{"Persistent state matters"},
	}
	lens := &schema.LensResult{
		LensID:         "openclaw-product",
		RelevanceScore: 0.9,
		Rationale:      "Highly relevant to agent orchestration.",
		RankedIdeas: []schema.RankedIdea{
			{Title: "Persistent threads", Score: 0.9, WhyItMatters: "Stateful agents", ImplementationFit: "High", Rationale: "Core architecture"},
			{Title: "Concurrent tools", Score: 0.8, WhyItMatters: "Lower latency", ImplementationFit: "Medium", Rationale: "Parallelism"},
		},
		RecommendedArtifacts: []string{"Create PRD", "Write task list"},
	}
	return src, ext, lens
}

func TestRenderFormats(t *testing.T) {
	src, ext, lens := sampleRecords()
	cases := map[string]string{
		FormatVault:   "Breakdown",
		FormatSummary: "Executive Summary",
		FormatPRD:     "PRD Seed",
		FormatTasks:   "Task List",
	}
	for format, want := range cases {
		got, err := Render(format, src, ext, lens)
		if err != nil {
			t.Fatalf("Render(%s) error: %v", format, err)
		}
		if !strings.Contains(got, want) {
			t.Fatalf("Render(%s) missing %q:\n%s", format, want, got)
		}
	}
}

func TestRenderRejectsUnknownFormat(t *testing.T) {
	src, ext, lens := sampleRecords()
	if _, err := Render("wat", src, ext, lens); err == nil {
		t.Fatal("expected error for unknown format")
	}
}
