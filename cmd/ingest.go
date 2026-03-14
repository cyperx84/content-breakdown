// Package cmd contains CLI commands for the breakdown tool.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/cyperx84/content-breakdown/internal/schema"
	"github.com/cyperx84/content-breakdown/internal/youtube"
)

var ingestCmd = &cobra.Command{
	Use:   "ingest <url>",
	Short: "Ingest a source URL and produce source.json",
	Long: `Ingest a source URL (YouTube for MVP) and produce a normalized SourceRecord.

The source record is written to the artifacts directory as source.json.
Use --json to output the record to stdout instead.`,
	Args: cobra.ExactArgs(1),
	RunE: runIngest,
}

var (
	ingestArtifactsDir string
	ingestJSONOutput   bool
)

func init() {
	rootCmd.AddCommand(ingestCmd)
	ingestCmd.Flags().StringVar(&ingestArtifactsDir, "artifacts-dir", "", "Artifacts directory (default: ./artifacts/content-breakdown/<slug>/)")
	ingestCmd.Flags().BoolVar(&ingestJSONOutput, "json", false, "Output SourceRecord as JSON to stdout")
}

func runIngest(cmd *cobra.Command, args []string) error {
	url := args[0]

	// Detect source type (YouTube only for MVP)
	if !isYouTubeURL(url) {
		return fmt.Errorf("unsupported source type (only YouTube URLs supported in MVP)")
	}

	// Ingest via YouTube adapter
	record, err := youtube.Ingest(url)
	if err != nil {
		return fmt.Errorf("ingest failed: %w", err)
	}

	// Determine artifacts directory
	artifactDir := ingestArtifactsDir
	if artifactDir == "" {
		slug := generateSlug(record)
		artifactDir = filepath.Join("artifacts", "content-breakdown", slug)
	}

	// Ensure directory exists
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return fmt.Errorf("create artifacts dir: %w", err)
	}

	// Write source.json
	sourcePath := filepath.Join(artifactDir, "source.json")
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal source record: %w", err)
	}

	if err := os.WriteFile(sourcePath, data, 0644); err != nil {
		return fmt.Errorf("write source.json: %w", err)
	}

	// Output
	if ingestJSONOutput {
		fmt.Println(string(data))
	} else {
		fmt.Fprintf(os.Stderr, "Ingested: %s\n", record.Title)
		fmt.Fprintf(os.Stderr, "Artifacts: %s\n", artifactDir)
		fmt.Fprintf(os.Stderr, "Source: %s\n", sourcePath)
	}

	return nil
}

func isYouTubeURL(url string) bool {
	return containsAny(url, "youtube.com/watch", "youtu.be/", "youtube.com/shorts")
}

func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func generateSlug(record *schema.SourceRecord) string {
	date := time.Now().Format("2006-01-02")
	titleSlug := youtube.Slug(record.Title)
	return fmt.Sprintf("%s_%s", date, titleSlug)
}
