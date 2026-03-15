package schema

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSourceRecordJSON(t *testing.T) {
	now := time.Now()
	published := "2026-01-15"
	duration := "10m30s"

	src := SourceRecord{
		ID:           "yt_abc123",
		Type:         "youtube",
		CanonicalURL: "https://youtube.com/watch?v=abc123",
		Title:        "Test Video",
		Author:       "Test Author",
		PublishedAt:  &published,
		Duration:     &duration,
		Transcript:   "This is a test transcript.",
		Metadata: SourceMetadata{
			ExtractedAt: now,
			Extractor:   "yt-dlp",
			VideoID:     "abc123",
		},
	}

	data, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got SourceRecord
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != src.ID {
		t.Errorf("ID = %q, want %q", got.ID, src.ID)
	}
	if got.Title != src.Title {
		t.Errorf("Title = %q, want %q", got.Title, src.Title)
	}
	if got.PublishedAt == nil || *got.PublishedAt != published {
		t.Errorf("PublishedAt = %v, want %q", got.PublishedAt, published)
	}
}

func TestExtractionRecordJSON(t *testing.T) {
	ext := ExtractionRecord{
		SourceID:      "yt_abc123",
		Summary:       "Test summary.",
		Tools:         []string{"Tool1", "Tool2"},
		Workflows:     []string{"Workflow1"},
		Opportunities: []string{"Opp1"},
		Claims:        []string{"Claim1"},
		Quotes:        []string{"Quote1"},
		Metadata: ExtractionMetadata{
			GeneratedAt: time.Now(),
		},
	}

	data, err := json.MarshalIndent(ext, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ExtractionRecord
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.SourceID != ext.SourceID {
		t.Errorf("SourceID = %q, want %q", got.SourceID, ext.SourceID)
	}
	if len(got.Tools) != len(ext.Tools) {
		t.Errorf("Tools count = %d, want %d", len(got.Tools), len(ext.Tools))
	}
}

func TestLensResultJSON(t *testing.T) {
	lens := LensResult{
		SourceID:       "yt_abc123",
		LensID:         "openclaw-product",
		RelevanceScore: 0.85,
		Rationale:      "Highly relevant for product development.",
		RankedIdeas: []RankedIdea{
			{
				Title:             "Idea 1",
				Rationale:         "Why it's good",
				WhyItMatters:      "Impact",
				ImplementationFit: "High",
				Score:             0.9,
			},
		},
		RecommendedArtifacts: []string{"PRD", "Task list"},
		IgnoredItems:         []string{"Hype"},
		Metadata: LensMetadata{
			GeneratedAt: time.Now(),
		},
	}

	data, err := json.MarshalIndent(lens, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got LensResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.LensID != lens.LensID {
		t.Errorf("LensID = %q, want %q", got.LensID, lens.LensID)
	}
	if got.RelevanceScore != lens.RelevanceScore {
		t.Errorf("RelevanceScore = %f, want %f", got.RelevanceScore, lens.RelevanceScore)
	}
	if len(got.RankedIdeas) != len(lens.RankedIdeas) {
		t.Errorf("RankedIdeas count = %d, want %d", len(got.RankedIdeas), len(lens.RankedIdeas))
	}
}

func TestArtifactManifestJSON(t *testing.T) {
	manifest := ArtifactManifest{
		SourceID: "yt_abc123",
		LensID:   "openclaw-product",
		Emitted: []EmittedArtifact{
			{Type: "vault-note", Path: "/tmp/note.md"},
			{Type: "prd", Path: "/tmp/prd.md"},
		},
		CreatedAt: time.Now(),
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ArtifactManifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.SourceID != manifest.SourceID {
		t.Errorf("SourceID = %q, want %q", got.SourceID, manifest.SourceID)
	}
	if len(got.Emitted) != len(manifest.Emitted) {
		t.Errorf("Emitted count = %d, want %d", len(got.Emitted), len(manifest.Emitted))
	}
}

func TestRankedIdeaJSON(t *testing.T) {
	idea := RankedIdea{
		Title:             "Test Idea",
		Rationale:         "Test rationale",
		WhyItMatters:      "Test impact",
		ImplementationFit: "High",
		Score:             0.85,
	}

	data, err := json.Marshal(idea)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got RankedIdea
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Title != idea.Title {
		t.Errorf("Title = %q, want %q", got.Title, idea.Title)
	}
	if got.Score != idea.Score {
		t.Errorf("Score = %f, want %f", got.Score, idea.Score)
	}
}
