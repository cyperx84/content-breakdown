# breakdown

Content Breakdown Workflow — transforms source material into structured findings, lens-based synthesis, and actionable vault notes.

## Status

MVP scaffold. Core types and package layout are in place. Extraction and lens pipeline implementation pending.

## Prerequisites

- Go 1.26+
- `yt-dlp` on PATH (`brew install yt-dlp`)

## Package Layout

```
breakdown/
├── main.go                      # Entry point
├── cmd/                         # Cobra CLI commands
│   ├── root.go
│   ├── version.go
│   ├── run.go                   # (todo) Happy-path orchestration
│   ├── ingest.go                # (todo) Source ingestion
│   ├── analyze.go               # (todo) Extraction + lens
│   └── emit.go                  # (todo) Artifact emission
├── internal/
│   ├── schema/record.go         # SourceRecord, ExtractionRecord, LensResult
│   ├── youtube/ingest.go        # yt-dlp wrapper + VTT parser
│   ├── extract/                 # (todo) LLM extraction pass
│   ├── lens/                    # (todo) Lens execution
│   └── emit/vault.go            # Vault note markdown generation
├── lenses/
│   └── openclaw-product.json    # Lens definition
└── go.mod
```

## Usage (planned)

```bash
# Full pipeline
breakdown run "https://youtube.com/watch?v=..." --stdout

# Step-by-step
breakdown ingest "https://youtube.com/watch?v=..." --json
breakdown analyze ./artifacts/content-breakdown/slug --lens openclaw-product
breakdown emit ./artifacts/content-breakdown/slug --stdout
```

## Architecture Notes

- **2 LLM calls per run:** extract → lens (emitter is pure template)
- **Stdin-mode LLM:** CLI emits prompts, harness pipes model responses
- **No API keys in CLI:** keyless design, harness provides model access
- **Artifacts on disk:** `artifacts/content-breakdown/<slug>/`

See `content-breakdown-mvp-review.md` in the workspace root for the full architecture review.
