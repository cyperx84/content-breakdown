package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cyperx84/content-breakdown/internal/emit"
	"github.com/cyperx84/content-breakdown/internal/lens"
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
Use --format to select output format: vault|summary|prd|tasks
Use --think to append a Mental Models section via the optional 'lattice' CLI.`,
	Args: cobra.ExactArgs(1),
	RunE: runPipelineCmd,
}

var (
	runLens         string
	runLLMCmd       string
	runArtifactsDir string
	runStdout       bool
	runVerbose      bool
	runFormat       string
	runThink        bool
	runChunkChars   int
)

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&runLens, "lens", "openclaw-product", "Lens ID to apply")
	runCmd.Flags().StringVar(&runLLMCmd, "llm-cmd", "", "External LLM command (e.g., 'claude --print --permission-mode bypassPermissions')")
	runCmd.Flags().StringVar(&runArtifactsDir, "artifacts-dir", "", "Base artifacts directory (default: ./artifacts/content-breakdown/)")
	runCmd.Flags().BoolVar(&runStdout, "stdout", false, "Output final note to stdout")
	runCmd.Flags().BoolVar(&runVerbose, "verbose", false, "Show progress on stderr")
	runCmd.Flags().StringVar(&runFormat, "format", emit.FormatVault, "Output format: vault|summary|prd|tasks")
	runCmd.Flags().BoolVar(&runThink, "think", false, "Append Mental Models analysis via the lattice CLI (requires 'lattice' on PATH)")
	runCmd.Flags().IntVar(&runChunkChars, "chunk-chars", 0, "Per-chunk transcript character cap for the extraction LLM (0 = default)")
}

func runPipelineCmd(cmd *cobra.Command, args []string) error {
	lensPath := findLens(runLens)
	if lensPath == "" {
		return fmt.Errorf("lens not found: %s", runLens)
	}
	lensDef, err := lens.LoadLens(lensPath)
	if err != nil {
		return fmt.Errorf("load lens: %w", err)
	}

	result, err := RunPipeline(PipelineOptions{
		Input:        args[0],
		Lens:         lensDef,
		LLMCmd:       runLLMCmd,
		Format:       runFormat,
		ArtifactsDir: runArtifactsDir,
		Stdout:       runStdout,
		Verbose:      runVerbose,
		Think:        runThink,
		ChunkChars:   runChunkChars,
	})
	if err != nil {
		return err
	}

	if runVerbose || !runStdout {
		fmt.Fprintf(os.Stderr, "[run] Complete! Relevance: %.2f | Ideas: %d\n",
			result.LensResult.RelevanceScore, len(result.LensResult.RankedIdeas))
	}
	return nil
}
