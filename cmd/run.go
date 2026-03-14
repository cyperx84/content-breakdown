// Package cmd contains CLI commands for the breakdown tool.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/cyperx84/content-breakdown/internal/emit"
	"github.com/cyperx84/content-breakdown/internal/extract"
	"github.com/cyperx84/content-breakdown/internal/lens"
	"github.com/cyperx84/content-breakdown/internal/schema"
	"github.com/cyperx84/content-breakdown/internal/youtube"
)

var runCmd = &cobra.Command{
	Use:   "run <url>",
	Short: "Full pipeline: ingest → analyze → emit",
	Long: `Run the complete breakdown pipeline on a source URL.

This orchestrates the full workflow:
  1. Ingest: Fetch source (YouTube) and produce source.json
  2. Analyze: Extract findings and apply lens
  3. Emit: Generate vault note

Use --stdout to output the final markdown note to stdout.
Use --artifacts-dir to specify where intermediate files are stored.`,
	Args: cobra.ExactArgs(1),
	RunE: runPipeline,
}

var (
	runLens         string
	runLLMCmd       string
	runArtifactsDir string
	runStdout       bool
	runVerbose      bool
)

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&runLens, "lens", "openclaw-product", "Lens ID to apply")
	runCmd.Flags().StringVar(&runLLMCmd, "llm-cmd", "", "External LLM command (e.g., 'claude -p')")
	runCmd.Flags().StringVar(&runArtifactsDir, "artifacts-dir", "", "Artifacts directory (default: ./artifacts/content-breakdown/<slug>/)")
	runCmd.Flags().BoolVar(&runStdout, "stdout", false, "Output final markdown note to stdout")
	runCmd.Flags().BoolVar(&runVerbose, "verbose", false, "Show progress on stderr")
}

func runPipeline(cmd *cobra.Command, args []string) error {
	url := args[0]

	// Validate source type
	if !isYouTubeURL(url) {
		return fmt.Errorf("unsupported source type (only YouTube URLs supported in MVP)")
	}

	// Find lens definition
	lensPath := findLens(runLens)
	if lensPath == "" {
		return fmt.Errorf("lens not found: %s", runLens)
	}

	lensDef, err := lens.LoadLens(lensPath)
	if err != nil {
		return fmt.Errorf("load lens: %w", err)
	}

	// === STAGE 1: INGEST ===
	if runVerbose {
		fmt.Fprintf(os.Stderr, "[run] Stage 1/3: Ingesting...\n")
	}

	src, err := youtube.Ingest(url)
	if err != nil {
		return fmt.Errorf("ingest: %w", err)
	}

	// Determine artifacts directory
	artifactDir := runArtifactsDir
	if artifactDir == "" {
		artifactDir = filepath.Join("artifacts", "content-breakdown", generateSlug(src))
	}

	// Ensure directory exists
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return fmt.Errorf("create artifacts dir: %w", err)
	}

	// Write source.json
	if err := writeArtifact(filepath.Join(artifactDir, "source.json"), src); err != nil {
		return fmt.Errorf("write source.json: %w", err)
	}

	if runVerbose {
		fmt.Fprintf(os.Stderr, "[run] Wrote: %s\n", filepath.Join(artifactDir, "source.json"))
	}

	// === STAGE 2: ANALYZE (extract + lens) ===
	if runVerbose {
		fmt.Fprintf(os.Stderr, "[run] Stage 2/3: Analyzing...\n")
	}

	// Extraction pass
	extractOpts := extract.Options{
		LLMCmd:  runLLMCmd,
		Verbose: runVerbose,
	}

	extRecord, err := extract.Run(src, extractOpts)
	if err != nil {
		return fmt.Errorf("extraction: %w", err)
	}

	if err := writeArtifact(filepath.Join(artifactDir, "extraction.json"), extRecord); err != nil {
		return fmt.Errorf("write extraction.json: %w", err)
	}

	if runVerbose {
		fmt.Fprintf(os.Stderr, "[run] Wrote: %s\n", filepath.Join(artifactDir, "extraction.json"))
	}

	// Lens pass
	lensOpts := lens.Options{
		LLMCmd:  runLLMCmd,
		Verbose: runVerbose,
	}

	lensResult, err := lens.Run(src, extRecord, lensDef, lensOpts)
	if err != nil {
		return fmt.Errorf("lens: %w", err)
	}

	if err := writeArtifact(filepath.Join(artifactDir, "lens.json"), lensResult); err != nil {
		return fmt.Errorf("write lens.json: %w", err)
	}

	if runVerbose {
		fmt.Fprintf(os.Stderr, "[run] Wrote: %s\n", filepath.Join(artifactDir, "lens.json"))
	}

	// === STAGE 3: EMIT ===
	if runVerbose {
		fmt.Fprintf(os.Stderr, "[run] Stage 3/3: Emitting...\n")
	}

	note := emit.VaultNote(src, extRecord, lensResult)

	manifest := &schema.ArtifactManifest{
		SourceID:  src.ID,
		LensID:    lensResult.LensID,
		CreatedAt: time.Now(),
	}

	if runStdout {
		fmt.Print(note)
		manifest.Emitted = append(manifest.Emitted, schema.EmittedArtifact{Type: "stdout", Path: "stdout"})
	} else {
		notePath := filepath.Join(artifactDir, "note.md")
		if err := os.WriteFile(notePath, []byte(note), 0644); err != nil {
			return fmt.Errorf("write note.md: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Wrote: %s\n", notePath)
		manifest.Emitted = append(manifest.Emitted, schema.EmittedArtifact{Type: "vault-note", Path: notePath})
	}

	if err := writeManifest(artifactDir, manifest); err != nil {
		return err
	}

	if runVerbose || !runStdout {
		fmt.Fprintf(os.Stderr, "[run] Complete! Relevance: %.2f | Ideas: %d\n",
			lensResult.RelevanceScore, len(lensResult.RankedIdeas))
	}

	return nil
}

func writeArtifact(path string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, jsonData, 0644)
}
