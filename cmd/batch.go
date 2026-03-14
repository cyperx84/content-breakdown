// Package cmd contains CLI commands for the breakdown tool.
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
	"github.com/cyperx84/content-breakdown/internal/extract"
	"github.com/cyperx84/content-breakdown/internal/lens"
	"github.com/cyperx84/content-breakdown/internal/schema"
	"github.com/cyperx84/content-breakdown/internal/source"
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
			r.ArtDir, r.Title, r.Err = processSingle(src, lensDef)
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

	// Print summary
	ok, failed := 0, 0
	fmt.Fprintln(os.Stderr, "\n── Batch Summary ─────────────────────")
	for _, r := range results {
		if r.Err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "  ✗  %s\n     error: %v\n", r.Input, r.Err)
			if !batchSkipErrors {
				return fmt.Errorf("batch failed on: %s: %w", r.Input, r.Err)
			}
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

func processSingle(input string, lensDef *lens.LensDefinition) (artDir string, title string, err error) {
	src, err := source.Ingest(input)
	if err != nil {
		return "", "", fmt.Errorf("ingest: %w", err)
	}

	artDir = fmt.Sprintf("%s/%s", batchArtifactDir, generateSlug(src))
	if err := os.MkdirAll(artDir, 0755); err != nil {
		return artDir, src.Title, fmt.Errorf("mkdir: %w", err)
	}

	if err := writeArtifact(artDir+"/source.json", src); err != nil {
		return artDir, src.Title, err
	}

	extRecord, err := extract.Run(src, extract.Options{LLMCmd: batchLLMCmd, Verbose: batchVerbose})
	if err != nil {
		return artDir, src.Title, fmt.Errorf("extraction: %w", err)
	}
	if err := writeArtifact(artDir+"/extraction.json", extRecord); err != nil {
		return artDir, src.Title, err
	}

	lensResult, err := lens.Run(src, extRecord, lensDef, lens.Options{LLMCmd: batchLLMCmd, Verbose: batchVerbose})
	if err != nil {
		return artDir, src.Title, fmt.Errorf("lens: %w", err)
	}
	if err := writeArtifact(artDir+"/lens.json", lensResult); err != nil {
		return artDir, src.Title, err
	}

	rendered, err := emit.Render(batchFormat, src, extRecord, lensResult)
	if err != nil {
		return artDir, src.Title, fmt.Errorf("render: %w", err)
	}

	fname := batchFormat + ".md"
	if batchFormat == emit.FormatVault {
		fname = "note.md"
	}
	if err := os.WriteFile(artDir+"/"+fname, []byte(rendered), 0644); err != nil {
		return artDir, src.Title, fmt.Errorf("write note: %w", err)
	}

	manifest := &schema.ArtifactManifest{
		SourceID:  src.ID,
		LensID:    lensResult.LensID,
		Emitted:   []schema.EmittedArtifact{{Type: batchFormat, Path: artDir + "/" + fname}},
		CreatedAt: time.Now(),
	}
	_ = writeArtifact(artDir+"/manifest.json", manifest)

	return artDir, src.Title, nil
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
