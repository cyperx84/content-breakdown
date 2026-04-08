package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cyperx84/content-breakdown/internal/emit"
	"github.com/cyperx84/content-breakdown/internal/extract"
	"github.com/cyperx84/content-breakdown/internal/lens"
	"github.com/cyperx84/content-breakdown/internal/schema"
	"github.com/cyperx84/content-breakdown/internal/source"
)

// PipelineOptions configures a single end-to-end run.
type PipelineOptions struct {
	Input        string
	Lens         *lens.LensDefinition
	LLMCmd       string
	Format       string
	ArtifactsDir string // base dir; the per-source slug is appended
	Stdout       bool
	Verbose      bool
	Think        bool
	ChunkChars   int
}

// PipelineResult holds the outputs of a successful pipeline run.
type PipelineResult struct {
	ArtifactDir string
	Source      *schema.SourceRecord
	Extraction  *schema.ExtractionRecord
	LensResult  *schema.LensResult
	Rendered    string
	Manifest    *schema.ArtifactManifest
}

// RunPipeline performs ingest → extract → lens → emit and writes artifacts.
// Both `run` and `batch` commands route through this function so the
// workflow lives in exactly one place.
func RunPipeline(opts PipelineOptions) (*PipelineResult, error) {
	if opts.Lens == nil {
		return nil, fmt.Errorf("pipeline: lens is required")
	}
	if opts.Format == "" {
		opts.Format = emit.FormatVault
	}

	// 1. Ingest
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "[run] Stage 1/3: Ingesting %s...\n", opts.Input)
	}
	src, err := source.Ingest(opts.Input)
	if err != nil {
		return nil, fmt.Errorf("ingest: %w", err)
	}
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "[run] Ingested: %s (%s)\n", src.Title, src.Type)
	}

	artifactDir := opts.ArtifactsDir
	if artifactDir == "" {
		artifactDir = filepath.Join("artifacts", "content-breakdown")
	}
	artifactDir = filepath.Join(artifactDir, generateSlug(src))

	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return nil, fmt.Errorf("create artifacts dir: %w", err)
	}
	if err := writeJSON(filepath.Join(artifactDir, "source.json"), src); err != nil {
		return nil, fmt.Errorf("write source.json: %w", err)
	}

	// 2. Extract
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "[run] Stage 2/3: Analyzing...\n")
	}
	extRecord, err := extract.Run(src, extract.Options{
		LLMCmd:             opts.LLMCmd,
		Verbose:            opts.Verbose,
		MaxTranscriptChars: opts.ChunkChars,
	})
	if err != nil {
		return nil, fmt.Errorf("extraction: %w", err)
	}
	if err := writeJSON(filepath.Join(artifactDir, "extraction.json"), extRecord); err != nil {
		return nil, fmt.Errorf("write extraction.json: %w", err)
	}

	// 3. Lens
	lensResult, err := lens.Run(src, extRecord, opts.Lens, lens.Options{
		LLMCmd:  opts.LLMCmd,
		Verbose: opts.Verbose,
	})
	if err != nil {
		return nil, fmt.Errorf("lens: %w", err)
	}
	if err := writeJSON(filepath.Join(artifactDir, "lens.json"), lensResult); err != nil {
		return nil, fmt.Errorf("write lens.json: %w", err)
	}

	// 4. Emit
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "[run] Stage 3/3: Emitting (%s)...\n", opts.Format)
	}
	rendered, err := emit.Render(opts.Format, src, extRecord, lensResult)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	if opts.Think {
		if section := runLatticeThink(src.Title, opts.Verbose); section != "" {
			rendered += "\n" + section
		}
	}

	manifest := loadOrInitManifest(artifactDir, src.ID, lensResult.LensID)

	if opts.Stdout {
		fmt.Print(rendered)
		recordEmittedArtifact(manifest, opts.Format, "stdout")
	} else {
		fname := opts.Format + ".md"
		if opts.Format == emit.FormatVault {
			fname = "note.md"
		}
		notePath := filepath.Join(artifactDir, fname)
		if err := os.WriteFile(notePath, []byte(rendered), 0644); err != nil {
			return nil, fmt.Errorf("write %s: %w", fname, err)
		}
		fmt.Fprintf(os.Stderr, "Wrote: %s\n", notePath)
		recordEmittedArtifact(manifest, opts.Format, notePath)
	}

	if err := writeManifest(artifactDir, manifest); err != nil {
		return nil, err
	}

	return &PipelineResult{
		ArtifactDir: artifactDir,
		Source:      src,
		Extraction:  extRecord,
		LensResult:  lensResult,
		Rendered:    rendered,
		Manifest:    manifest,
	}, nil
}

// runLatticeThink invokes the optional `lattice` CLI to append a Mental Models
// section. If lattice isn't on PATH or fails, the section is silently skipped
// (only logged when verbose).
func runLatticeThink(contentTitle string, verbose bool) string {
	if _, err := exec.LookPath("lattice"); err != nil {
		if verbose {
			fmt.Fprintln(os.Stderr, "[run] lattice not on PATH, skipping --think")
		}
		return ""
	}
	if verbose {
		fmt.Fprintln(os.Stderr, "[run] Running lattice think...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lattice", "think", contentTitle, "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[run] Lattice failed: %s (%s)\n", err, stderr.String())
		}
		return ""
	}

	var result struct {
		Models []struct {
			ModelName string `json:"model_name"`
			Category  string `json:"category"`
		} `json:"models"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[run] Lattice JSON parse failed: %s\n", err)
		}
		return ""
	}

	var b bytes.Buffer
	b.WriteString("## Mental Models\n\n")
	for _, m := range result.Models {
		fmt.Fprintf(&b, "- **%s** (%s)\n", m.ModelName, m.Category)
	}
	if result.Summary != "" {
		b.WriteString("\n### Synthesis\n\n")
		b.WriteString(result.Summary)
		b.WriteString("\n")
	}
	return b.String()
}
