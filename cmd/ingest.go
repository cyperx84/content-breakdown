// Package cmd contains CLI commands for the breakdown tool.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cyperx84/content-breakdown/internal/schema"
	"github.com/cyperx84/content-breakdown/internal/source"
	_ "github.com/cyperx84/content-breakdown/internal/source" // register all adapters
	"github.com/cyperx84/content-breakdown/internal/youtube"
)

var ingestCmd = &cobra.Command{
	Use:   "ingest <url-or-file>",
	Short: "Ingest a source and produce source.json",
	Long: `Ingest a source (YouTube URL, article URL, or local file) and produce a normalized SourceRecord.

Supported sources:
  - YouTube URLs (youtube.com, youtu.be)
  - Article / webpage URLs (http/https)
  - Local files (.md, .txt, .pdf)

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
	input := args[0]

	record, err := source.Ingest(input)
	if err != nil {
		return fmt.Errorf("ingest failed: %w", err)
	}

	artifactDir := ingestArtifactsDir
	if artifactDir == "" {
		artifactDir = filepath.Join("artifacts", "content-breakdown", generateSlug(record))
	}

	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return fmt.Errorf("create artifacts dir: %w", err)
	}

	sourcePath := filepath.Join(artifactDir, "source.json")
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal source record: %w", err)
	}

	if err := os.WriteFile(sourcePath, data, 0644); err != nil {
		return fmt.Errorf("write source.json: %w", err)
	}

	if ingestJSONOutput {
		fmt.Println(string(data))
	} else {
		fmt.Fprintf(os.Stderr, "Ingested: %s\n", record.Title)
		fmt.Fprintf(os.Stderr, "Type: %s\n", record.Type)
		fmt.Fprintf(os.Stderr, "Artifacts: %s\n", artifactDir)
	}

	return nil
}

func generateSlug(record *schema.SourceRecord) string {
	date := time.Now().Format("2006-01-02")
	titleSlug := titleToSlug(record.Title)
	return fmt.Sprintf("%s_%s", date, titleSlug)
}

func titleToSlug(title string) string {
	// Reuse youtube.Slug logic for any title
	return youtube.Slug(title)
}

// isYouTubeURL is kept for use in cmd/run.go compatibility
func isYouTubeURL(u string) bool {
	return containsAny(u, "youtube.com/watch", "youtu.be/", "youtube.com/shorts")
}

func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
