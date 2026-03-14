// Package cmd contains CLI commands for the breakdown tool.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cyperx84/content-breakdown/internal/extract"
	"github.com/cyperx84/content-breakdown/internal/lens"
	"github.com/cyperx84/content-breakdown/internal/schema"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze <artifacts-dir>",
	Short: "Run extraction and lens passes on ingested source",
	Long: `Run the extraction and lens passes on a previously ingested source.

Reads source.json from the artifacts directory, runs the extraction pass
(tools, workflows, opportunities), then applies the lens to produce ranked insights.

Outputs:
  - extraction.json (structured findings)
  - lens.json (ranked, lens-specific insights)`,
	Args: cobra.ExactArgs(1),
	RunE: runAnalyze,
}

var (
	analyzeLens     string
	analyzeLLMCmd   string
	analyzeVerbose  bool
	analyzeJSONOutput bool
)

func init() {
	rootCmd.AddCommand(analyzeCmd)
	analyzeCmd.Flags().StringVar(&analyzeLens, "lens", "openclaw-product", "Lens ID to apply")
	analyzeCmd.Flags().StringVar(&analyzeLLMCmd, "llm-cmd", "", "External LLM command (e.g., 'claude -p')")
	analyzeCmd.Flags().BoolVar(&analyzeVerbose, "verbose", false, "Show progress on stderr")
	analyzeCmd.Flags().BoolVar(&analyzeJSONOutput, "json", false, "Output LensResult as JSON to stdout")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	artifactDir := args[0]

	// Load source.json
	sourcePath := filepath.Join(artifactDir, "source.json")
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read source.json: %w (run 'breakdown ingest' first)", err)
	}

	var src schema.SourceRecord
	if err := json.Unmarshal(sourceData, &src); err != nil {
		return fmt.Errorf("parse source.json: %w", err)
	}

	// Find lens definition
	lensPath := findLens(analyzeLens)
	if lensPath == "" {
		return fmt.Errorf("lens not found: %s (checked ./lenses/ and embedded lenses)", analyzeLens)
	}

	lensDef, err := lens.LoadLens(lensPath)
	if err != nil {
		return fmt.Errorf("load lens: %w", err)
	}

	// Run extraction pass
	extractOpts := extract.Options{
		LLMCmd:  analyzeLLMCmd,
		Verbose: analyzeVerbose,
	}

	extRecord, err := extract.Run(&src, extractOpts)
	if err != nil {
		return fmt.Errorf("extraction pass: %w", err)
	}

	// Write extraction.json
	extPath := filepath.Join(artifactDir, "extraction.json")
	extData, err := json.MarshalIndent(extRecord, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal extraction record: %w", err)
	}

	if err := os.WriteFile(extPath, extData, 0644); err != nil {
		return fmt.Errorf("write extraction.json: %w", err)
	}

	if analyzeVerbose {
		fmt.Fprintf(os.Stderr, "[analyze] Wrote: %s\n", extPath)
	}

	// Run lens pass
	lensOpts := lens.Options{
		LLMCmd:  analyzeLLMCmd,
		Verbose: analyzeVerbose,
	}

	lensResult, err := lens.Run(&src, extRecord, lensDef, lensOpts)
	if err != nil {
		return fmt.Errorf("lens pass: %w", err)
	}

	// Write lens.json
	lensPath = filepath.Join(artifactDir, "lens.json")
	lensData, err := json.MarshalIndent(lensResult, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lens result: %w", err)
	}

	if err := os.WriteFile(lensPath, lensData, 0644); err != nil {
		return fmt.Errorf("write lens.json: %w", err)
	}

	if analyzeVerbose {
		fmt.Fprintf(os.Stderr, "[analyze] Wrote: %s\n", lensPath)
	}

	// Output
	if analyzeJSONOutput {
		fmt.Println(string(lensData))
	} else {
		fmt.Fprintf(os.Stderr, "Analyzed: %s\n", src.Title)
		fmt.Fprintf(os.Stderr, "Relevance: %.2f\n", lensResult.RelevanceScore)
		fmt.Fprintf(os.Stderr, "Ideas: %d ranked\n", len(lensResult.RankedIdeas))
		fmt.Fprintf(os.Stderr, "Artifacts: %s\n", artifactDir)
	}

	return nil
}

func findLens(lensID string) string {
	// Check ./lenses/<id>.json
	localPath := filepath.Join("lenses", lensID+".json")
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}

	// Check ~/.openclaw/lenses/<id>.json
	homeLensPath := filepath.Join(os.Getenv("HOME"), ".openclaw", "lenses", lensID+".json")
	if _, err := os.Stat(homeLensPath); err == nil {
		return homeLensPath
	}

	return ""
}
