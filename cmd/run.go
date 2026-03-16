// Package cmd contains CLI commands for the breakdown tool.
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/cyperx84/content-breakdown/internal/emit"
	"github.com/cyperx84/content-breakdown/internal/extract"
	"github.com/cyperx84/content-breakdown/internal/lens"
	"github.com/cyperx84/content-breakdown/internal/schema"
	"github.com/cyperx84/content-breakdown/internal/source"
	_ "github.com/cyperx84/content-breakdown/internal/source" // register adapters
)

var runCmd = &cobra.Command{
	Use:   "run <url-or-file>",
	Short: "Full pipeline: ingest → analyze → emit",
	Long: `Run the complete breakdown pipeline on a source.

Supported sources:
  - YouTube URLs
  - Article / webpage URLs
  - Local files (.md, .txt, .pdf)

Orchestrates: ingest → extract → lens → emit.

Use --stdout to output the final note to stdout.
Use --format to select output format: vault|summary|prd|tasks`,
	Args: cobra.ExactArgs(1),
	RunE: runPipeline,
}

var (
	runLens         string
	runLLMCmd       string
	runArtifactsDir string
	runStdout       bool
	runVerbose      bool
	runFormat       string
	runThink        bool
)

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&runLens, "lens", "openclaw-product", "Lens ID to apply")
	runCmd.Flags().StringVar(&runLLMCmd, "llm-cmd", "", "External LLM command (e.g., 'claude --print --permission-mode bypassPermissions')")
	runCmd.Flags().StringVar(&runArtifactsDir, "artifacts-dir", "", "Artifacts directory (default: ./artifacts/content-breakdown/<slug>/)")
	runCmd.Flags().BoolVar(&runStdout, "stdout", false, "Output final note to stdout")
	runCmd.Flags().BoolVar(&runVerbose, "verbose", false, "Show progress on stderr")
	runCmd.Flags().StringVar(&runFormat, "format", emit.FormatVault, "Output format: vault|summary|prd|tasks")
	runCmd.Flags().BoolVar(&runThink, "think", false, "Append mental models analysis via lattice")
}

func runPipeline(cmd *cobra.Command, args []string) error {
	input := args[0]

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

	src, err := source.Ingest(input)
	if err != nil {
		return fmt.Errorf("ingest: %w", err)
	}

	if runVerbose {
		fmt.Fprintf(os.Stderr, "[run] Ingested: %s (%s)\n", src.Title, src.Type)
	}

	// Determine artifacts directory
	artifactDir := runArtifactsDir
	if artifactDir == "" {
		artifactDir = filepath.Join("artifacts", "content-breakdown", generateSlug(src))
	}

	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return fmt.Errorf("create artifacts dir: %w", err)
	}

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
		fmt.Fprintf(os.Stderr, "[run] Stage 3/3: Emitting (%s)...\n", runFormat)
	}

	rendered, err := emit.Render(runFormat, src, extRecord, lensResult)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	manifest := &schema.ArtifactManifest{
		SourceID:  src.ID,
		LensID:    lensResult.LensID,
		CreatedAt: time.Now(),
	}

	// Lattice mental models integration
	if runThink {
		thinkSection := runLatticeThink(src.Title, runVerbose)
		if thinkSection != "" {
			rendered += "\n" + thinkSection
		}
	}

	if runStdout {
		fmt.Print(rendered)
		manifest.Emitted = append(manifest.Emitted, schema.EmittedArtifact{Type: runFormat, Path: "stdout"})
	} else {
		fname := runFormat + ".md"
		if runFormat == emit.FormatVault {
			fname = "note.md"
		}
		notePath := filepath.Join(artifactDir, fname)
		if err := os.WriteFile(notePath, []byte(rendered), 0644); err != nil {
			return fmt.Errorf("write %s: %w", fname, err)
		}
		fmt.Fprintf(os.Stderr, "Wrote: %s\n", notePath)
		manifest.Emitted = append(manifest.Emitted, schema.EmittedArtifact{Type: runFormat, Path: notePath})
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

type latticeThinkModel struct {
	ModelName string `json:"model_name"`
	Category  string `json:"category"`
}

type latticeThinkResult struct {
	Models  []latticeThinkModel `json:"models"`
	Summary string              `json:"summary"`
}

func runLatticeThink(contentTitle string, verbose bool) string {
	if _, err := exec.LookPath("lattice"); err != nil {
		fmt.Fprintf(os.Stderr, "[run] Warning: lattice not on PATH, skipping --think\n")
		return ""
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[run] Running lattice think...\n")
	}

	cmd := exec.Command("lattice", "think", contentTitle, "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[run] Lattice failed: %s\n", err)
		}
		return ""
	}

	var result latticeThinkResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[run] Lattice JSON parse failed: %s\n", err)
		}
		return ""
	}

	var section string
	section = "## Mental Models\n\n"
	for _, m := range result.Models {
		section += fmt.Sprintf("- **%s** (%s)\n", m.ModelName, m.Category)
	}
	if result.Summary != "" {
		section += "\n### Synthesis\n\n" + result.Summary + "\n"
	}

	return section
}
