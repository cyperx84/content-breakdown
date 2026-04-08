package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cyperx84/content-breakdown/internal/schema"
	"github.com/cyperx84/content-breakdown/internal/slug"
)

// generateSlug builds the per-source artifacts directory slug.
// Prefers the source's PublishedAt date when present so re-ingestion of the
// same content lands in the same directory; otherwise falls back to today.
func generateSlug(record *schema.SourceRecord) string {
	date := record.PublishedAt
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	return fmt.Sprintf("%s_%s", date, slug.Title(record.Title))
}

// findLens returns the path to a lens definition by id, searching ./lenses/
// then ~/.openclaw/lenses/. Returns "" if not found.
func findLens(lensID string) string {
	localPath := filepath.Join("lenses", lensID+".json")
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}
	homeLensPath := filepath.Join(os.Getenv("HOME"), ".openclaw", "lenses", lensID+".json")
	if _, err := os.Stat(homeLensPath); err == nil {
		return homeLensPath
	}
	return ""
}

// writeJSON writes any value as indented JSON to the given path.
func writeJSON(path string, data interface{}) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// loadOrInitManifest reads an existing manifest from the artifact dir, or
// returns a freshly initialized one if none exists. This lets multiple emit
// calls accumulate artifacts in the same manifest instead of overwriting.
func loadOrInitManifest(artifactDir, sourceID, lensID string) *schema.ArtifactManifest {
	path := filepath.Join(artifactDir, "manifest.json")
	if data, err := os.ReadFile(path); err == nil {
		var m schema.ArtifactManifest
		if json.Unmarshal(data, &m) == nil {
			// Update lens id when re-emitting under a different lens.
			m.LensID = lensID
			return &m
		}
	}
	return &schema.ArtifactManifest{
		SourceID:  sourceID,
		LensID:    lensID,
		CreatedAt: time.Now(),
	}
}

// recordEmittedArtifact appends a new artifact to a manifest, replacing any
// existing entry of the same type to avoid duplicates across re-runs.
func recordEmittedArtifact(m *schema.ArtifactManifest, artifactType, path string) {
	for i, e := range m.Emitted {
		if e.Type == artifactType {
			m.Emitted[i] = schema.EmittedArtifact{Type: artifactType, Path: path}
			return
		}
	}
	m.Emitted = append(m.Emitted, schema.EmittedArtifact{Type: artifactType, Path: path})
}

// writeManifest persists the manifest to <artifactDir>/manifest.json.
func writeManifest(artifactDir string, manifest *schema.ArtifactManifest) error {
	manifestPath := filepath.Join(artifactDir, "manifest.json")
	if err := writeJSON(manifestPath, manifest); err != nil {
		return fmt.Errorf("write manifest.json: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Wrote: %s\n", manifestPath)
	return nil
}
