package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cyperx84/content-breakdown/internal/emit"
	"github.com/cyperx84/content-breakdown/internal/schema"
)

var emitCmd = &cobra.Command{
	Use:   "emit <artifacts-dir>",
	Short: "Generate output artifacts from analysis artifacts",
	Long: `Generate a markdown artifact from the analysis artifacts.

Reads source.json, extraction.json, and lens.json from the artifacts
directory and produces a formatted markdown note.

Use --stdout to output to stdout, or --output to write to a file.`,
	Args: cobra.ExactArgs(1),
	RunE: runEmit,
}

var (
	emitStdout bool
	emitOutput string
	emitFormat string
)

func init() {
	rootCmd.AddCommand(emitCmd)
	emitCmd.Flags().BoolVar(&emitStdout, "stdout", false, "Output markdown to stdout")
	emitCmd.Flags().StringVar(&emitOutput, "output", "", "Output file path (default: <artifacts-dir>/<format>.md)")
	emitCmd.Flags().StringVar(&emitFormat, "format", emit.FormatVault, "Output format: vault|summary|prd|tasks")
}

func runEmit(cmd *cobra.Command, args []string) error {
	artifactDir := args[0]

	src, ext, lensResult, err := loadArtifacts(artifactDir)
	if err != nil {
		return err
	}

	rendered, err := emit.Render(emitFormat, src, ext, lensResult)
	if err != nil {
		return err
	}

	manifest := loadOrInitManifest(artifactDir, src.ID, lensResult.LensID)

	switch {
	case emitStdout:
		fmt.Print(rendered)
		recordEmittedArtifact(manifest, emitFormat, "stdout")
	case emitOutput != "":
		if err := os.WriteFile(emitOutput, []byte(rendered), 0644); err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Wrote: %s\n", emitOutput)
		recordEmittedArtifact(manifest, emitFormat, emitOutput)
	default:
		defaultName := emitFormat + ".md"
		if emitFormat == emit.FormatVault {
			defaultName = "note.md"
		}
		outPath := filepath.Join(artifactDir, defaultName)
		if err := os.WriteFile(outPath, []byte(rendered), 0644); err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Wrote: %s\n", outPath)
		recordEmittedArtifact(manifest, emitFormat, outPath)
	}

	return writeManifest(artifactDir, manifest)
}

func loadArtifacts(artifactDir string) (*schema.SourceRecord, *schema.ExtractionRecord, *schema.LensResult, error) {
	sourcePath := filepath.Join(artifactDir, "source.json")
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read source.json: %w", err)
	}

	var src schema.SourceRecord
	if err := json.Unmarshal(sourceData, &src); err != nil {
		return nil, nil, nil, fmt.Errorf("parse source.json: %w", err)
	}

	extPath := filepath.Join(artifactDir, "extraction.json")
	extData, err := os.ReadFile(extPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read extraction.json: %w (run 'breakdown analyze' first)", err)
	}

	var ext schema.ExtractionRecord
	if err := json.Unmarshal(extData, &ext); err != nil {
		return nil, nil, nil, fmt.Errorf("parse extraction.json: %w", err)
	}

	lensPath := filepath.Join(artifactDir, "lens.json")
	lensData, err := os.ReadFile(lensPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read lens.json: %w (run 'breakdown analyze' first)", err)
	}

	var lensResult schema.LensResult
	if err := json.Unmarshal(lensData, &lensResult); err != nil {
		return nil, nil, nil, fmt.Errorf("parse lens.json: %w", err)
	}

	return &src, &ext, &lensResult, nil
}
