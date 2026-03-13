// Package schema defines the machine-readable records for the Content Breakdown Workflow.
//
// These structs are the contract between pipeline stages. They are serialized
// to JSON for artifact storage and inter-process communication.
package schema

import "time"

// SourceRecord represents a normalized ingested source.
type SourceRecord struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	CanonicalURL string            `json:"canonicalUrl"`
	Title        string            `json:"title"`
	Author       string            `json:"author"`
	PublishedAt  *string           `json:"publishedAt,omitempty"`
	Duration     *string           `json:"duration,omitempty"`
	Transcript   string            `json:"transcript"`
	Metadata     SourceMetadata    `json:"metadata"`
}

// SourceMetadata holds extraction-time metadata.
type SourceMetadata struct {
	ExtractedAt time.Time `json:"extractedAt"`
	Extractor   string    `json:"extractor"`
	VideoID     string    `json:"videoId,omitempty"`
}

// ExtractionRecord represents structured findings from a source.
type ExtractionRecord struct {
	SourceID    string    `json:"sourceId"`
	Summary     string    `json:"summary"`
	Tools       []string  `json:"tools"`
	Workflows   []string  `json:"workflows"`
	Opportunities []string `json:"opportunities"`
	Claims      []string  `json:"claims,omitempty"`
	Quotes      []string  `json:"quotes,omitempty"`
	Metadata    ExtractionMetadata `json:"metadata"`
}

// ExtractionMetadata holds generation-time metadata.
type ExtractionMetadata struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Model       string    `json:"model,omitempty"`
}

// LensResult represents the output of a lens pass.
type LensResult struct {
	SourceID             string       `json:"sourceId"`
	LensID               string       `json:"lensId"`
	RelevanceScore       float64      `json:"relevanceScore"`
	Rationale            string       `json:"rationale"`
	RankedIdeas          []RankedIdea `json:"rankedIdeas"`
	RecommendedArtifacts []string     `json:"recommendedArtifacts"`
	IgnoredItems         []string     `json:"ignoredItems,omitempty"`
	Metadata             LensMetadata `json:"metadata"`
}

// RankedIdea represents a single ranked finding.
type RankedIdea struct {
	Title             string  `json:"title"`
	Rationale         string  `json:"rationale"`
	WhyItMatters      string  `json:"whyItMatters"`
	ImplementationFit string  `json:"implementationFit"`
	Score             float64 `json:"score"`
}

// LensMetadata holds lens execution metadata.
type LensMetadata struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Model       string    `json:"model,omitempty"`
}

// ArtifactManifest records what was emitted.
type ArtifactManifest struct {
	SourceID    string           `json:"sourceId"`
	LensID      string           `json:"lensId"`
	Emitted     []EmittedArtifact `json:"emitted"`
	CreatedAt   time.Time        `json:"createdAt"`
}

// EmittedArtifact records a single output artifact.
type EmittedArtifact struct {
	Type string `json:"type"`
	Path string `json:"path"`
}
