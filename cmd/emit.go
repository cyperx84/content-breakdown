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
	"github.com/cyperx84/content-breakdown/internal/schema"
)

var emitCmd = &cobra.Command{
	Use:   "emit <artifacts-dir>",
	Short: "Generate vault note from analysis artifacts",
	Long: `Generate a vault note (markdown) from the analysis artifacts.

Reads source.json, extraction.json, and lens.json from the artifacts
directory and produces a formatted markdown note.

Use --stdout to output to stdout, or --output to write to a file.`,
	Args: cobra.ExactArgs(1),
	RunE: runEmit,
}

var (
	emitStdout bool
	emitOutput string
)

func init() {
	rootCmd.AddCommand(emitCmd)
	emitCmd.Flags().BoolVar(&emitStdout, "stdout", false, "Output markdown to stdout")
	emitCmd.Flags().StringVar(&emitOutput, "output", "", "Output file path (default: <artifacts-dir>/note.md)")
}

func runEmit(cmd *cobra.Command, args []string) error {
	artifactDir := args[0]

	src, ext, lensResult, err := loadArtifacts(artifactDir)
	if err != nil {
		return err
	}

	note := emit.VaultNote(src, ext, lensResult)
	manifest := &schema.ArtifactManifest{
		SourceID:  src.ID,
		LensID:    lensResult.LensID,
		CreatedAt: time.Now(),
	}

	switch {
	case emitStdout:
		fmt.Print(note)
		manifest.Emitted = append(manifest.Emitted, schema.EmittedArtifact{Type: "stdout", Path: "stdout"})
	case emitOutput != "":
		if err := os.WriteFile(emitOutput, []byte(note), 0644); err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Wrote: %s\n", emitOutput)
		manifest.Emitted = append(manifest.Emitted, schema.EmittedArtifact{Type: "vault-note", Path: emitOutput})
	default:
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

	return nil
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

func writeManifest(artifactDir string, manifest *schema.ArtifactManifest) error {
	manifestPath := filepath.Join(artifactDir, "manifest.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("write manifest.json: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Wrote: %s\n", manifestPath)
	return nil
}
