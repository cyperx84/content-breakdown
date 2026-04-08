package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/cyperx84/content-breakdown/internal/emit"
	"github.com/cyperx84/content-breakdown/internal/lens"
)

var batchCmd = &cobra.Command{
	Use:   "batch [file]",
	Short: "Run the pipeline over multiple sources",
	Long: `Run the full pipeline on a list of source URLs or file paths.

Sources can be provided as:
  - A file with one URL/path per line (breakdown batch urls.txt)
  - stdin (cat urls.txt | breakdown batch)

Lines starting with '#' or empty lines are ignored.

Use --parallel to process sources concurrently (default 1 = sequential).
Use --skip-errors to continue on individual failures.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runBatch,
}

var (
	batchLens        string
	batchLLMCmd      string
	batchArtifactDir string
	batchFormat      string
	batchParallel    int
	batchSkipErrors  bool
	batchVerbose     bool
	batchChunkChars  int
)

func init() {
	rootCmd.AddCommand(batchCmd)
	batchCmd.Flags().StringVar(&batchLens, "lens", "openclaw-product", "Lens ID to apply")
	batchCmd.Flags().StringVar(&batchLLMCmd, "llm-cmd", "", "External LLM command")
	batchCmd.Flags().StringVar(&batchArtifactDir, "artifacts-dir", "artifacts/content-breakdown", "Base artifacts directory")
	batchCmd.Flags().StringVar(&batchFormat, "format", emit.FormatVault, "Output format: vault|summary|prd|tasks")
	batchCmd.Flags().IntVar(&batchParallel, "parallel", 1, "Number of sources to process concurrently")
	batchCmd.Flags().BoolVar(&batchSkipErrors, "skip-errors", false, "Continue on individual source failures")
	batchCmd.Flags().BoolVar(&batchVerbose, "verbose", false, "Show progress on stderr")
	batchCmd.Flags().IntVar(&batchChunkChars, "chunk-chars", 0, "Per-chunk transcript character cap (0 = default)")
}

type batchResult struct {
	Input    string
	ArtDir   string
	Title    string
	Err      error
	Duration time.Duration
}

func runBatch(cmd *cobra.Command, args []string) error {
	inputs, err := readInputList(args)
	if err != nil {
		return err
	}
	if len(inputs) == 0 {
		return fmt.Errorf("no inputs provided")
	}

	lensPath := findLens(batchLens)
	if lensPath == "" {
		return fmt.Errorf("lens not found: %s", batchLens)
	}
	lensDef, err := lens.LoadLens(lensPath)
	if err != nil {
		return fmt.Errorf("load lens: %w", err)
	}

	if batchParallel < 1 {
		batchParallel = 1
	}

	results := make([]batchResult, len(inputs))
	sem := make(chan struct{}, batchParallel)
	var wg sync.WaitGroup

	for i, input := range inputs {
		wg.Add(1)
		go func(idx int, src string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			r := batchResult{Input: src}
			res, err := RunPipeline(PipelineOptions{
				Input:        src,
				Lens:         lensDef,
				LLMCmd:       batchLLMCmd,
				Format:       batchFormat,
				ArtifactsDir: batchArtifactDir,
				Verbose:      batchVerbose,
				ChunkChars:   batchChunkChars,
			})
			if err != nil {
				r.Err = err
			} else {
				r.ArtDir = res.ArtifactDir
				r.Title = res.Source.Title
			}
			r.Duration = time.Since(start)
			results[idx] = r

			if r.Err != nil {
				fmt.Fprintf(os.Stderr, "[batch] FAIL (%s): %v\n", src, r.Err)
			} else if batchVerbose {
				fmt.Fprintf(os.Stderr, "[batch] OK   (%s) → %s  [%.1fs]\n", src, r.ArtDir, r.Duration.Seconds())
			}
		}(i, input)
	}
	wg.Wait()

	// Print summary (always — even when not skipping errors).
	ok, failed := 0, 0
	fmt.Fprintln(os.Stderr, "\n── Batch Summary ─────────────────────")
	for _, r := range results {
		if r.Err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "  ✗  %s\n     error: %v\n", r.Input, r.Err)
		} else {
			ok++
			fmt.Fprintf(os.Stderr, "  ✓  %s\n     → %s  [%.1fs]\n", r.Title, r.ArtDir, r.Duration.Seconds())
		}
	}
	fmt.Fprintf(os.Stderr, "──────────────────────────────────────\n")
	fmt.Fprintf(os.Stderr, "  Total: %d | OK: %d | Failed: %d\n", len(results), ok, failed)

	if failed > 0 && !batchSkipErrors {
		return fmt.Errorf("%d source(s) failed", failed)
	}
	return nil
}

func readInputList(args []string) ([]string, error) {
	var scanner *bufio.Scanner
	if len(args) == 1 {
		f, err := os.Open(args[0])
		if err != nil {
			return nil, fmt.Errorf("open input file: %w", err)
		}
		defer f.Close()
		scanner = bufio.NewScanner(f)
	} else {
		fi, _ := os.Stdin.Stat()
		if (fi.Mode() & os.ModeCharDevice) != 0 {
			return nil, fmt.Errorf("provide a file argument or pipe URLs to stdin")
		}
		scanner = bufio.NewScanner(os.Stdin)
	}

	var inputs []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		inputs = append(inputs, line)
	}
	return inputs, scanner.Err()
}
