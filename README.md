# breakdown

Content Breakdown Workflow — transforms source material into structured findings, lens-based synthesis, and actionable vault notes.

## Status

**Phases 1-5 complete.** Full pipeline working: ingest → extract → lens → emit.

Source types:
- YouTube URLs
- Article / webpage URLs
- Local files (`.md`, `.txt`, `.pdf`)

Output formats:
- `vault` — Obsidian-ready full note
- `summary` — executive summary
- `prd` — PRD seed doc
- `tasks` — task list

Lenses:
- `openclaw-product`
- `personal-os`
- `tooling-worth-stealing`
- `founder-research`

## Prerequisites

- Go 1.26+
- `yt-dlp` on PATH (`brew install yt-dlp`)

## Installation

```bash
go build -o breakdown .
```

Or install to $GOPATH/bin:
```bash
go install .
```

## Usage

### Full Pipeline (Recommended)

```bash
# Run the complete workflow with stdout output
breakdown run "https://youtube.com/watch?v=..." --stdout

# With verbose logging
breakdown run "https://youtube.com/watch?v=..." --stdout --verbose

# With custom LLM command
breakdown run "https://youtube.com/watch?v=..." --llm-cmd "claude -p" --stdout

# Specify custom artifacts directory
breakdown run "https://youtube.com/watch?v=..." --artifacts-dir ./my-artifacts
```

### Step-by-Step

```bash
# 1. Ingest source (YouTube)
breakdown ingest "https://youtube.com/watch?v=..." --json

# 2. Analyze (extract + lens)
breakdown analyze ./artifacts/content-breakdown/2026-03-14_video-title \
  --lens openclaw-product \
  --llm-cmd "claude -p"

# 3. Emit vault note
breakdown emit ./artifacts/content-breakdown/2026-03-14_video-title --stdout

# 4. Emit a PRD seed from the same artifacts
breakdown emit ./artifacts/content-breakdown/2026-03-14_video-title --format prd --stdout

# 5. Emit a task list from the same artifacts
breakdown emit ./artifacts/content-breakdown/2026-03-14_video-title --format tasks --stdout
```

## Commands

### `breakdown run <url-or-file>`

Full pipeline: ingest → analyze → emit.

Flags:
- `--lens string` - Lens ID to apply (default: "openclaw-product")
- `--llm-cmd string` - External LLM command (e.g., 'claude --print --permission-mode bypassPermissions')
- `--format string` - Output format: `vault|summary|prd|tasks` (default: `vault`)
- `--artifacts-dir string` - Artifacts directory (default: ./artifacts/content-breakdown/<slug>/)
- `--stdout` - Output final note to stdout
- `--verbose` - Show progress on stderr

### `breakdown batch [file]`

Run the pipeline over many sources from a file or stdin.

```bash
# From a file (one URL/path per line, # = comment)
breakdown batch urls.txt --llm-cmd 'claude --print --permission-mode bypassPermissions'

# From stdin
cat urls.txt | breakdown batch --skip-errors --parallel 2
```

Flags:
- `--lens string` - Lens ID (default: "openclaw-product")
- `--llm-cmd string` - External LLM command
- `--format string` - Output format (default: `vault`)
- `--parallel int` - Concurrent sources (default: 1)
- `--skip-errors` - Continue on individual failures
- `--artifacts-dir string` - Base artifacts directory
- `--verbose` - Show per-source progress

### `breakdown ingest <url-or-file>`

Ingest a source (YouTube URL, webpage URL, or local file) and produce `source.json`.

Flags:
- `--artifacts-dir string` - Artifacts directory
- `--json` - Output SourceRecord as JSON to stdout

### `breakdown analyze <artifacts-dir>`

Run extraction and lens passes on ingested source.

Flags:
- `--lens string` - Lens ID to apply (default: "openclaw-product")
- `--llm-cmd string` - External LLM command
- `--json` - Output LensResult as JSON to stdout
- `--verbose` - Show progress on stderr

### `breakdown emit <artifacts-dir>`

Generate output artifacts from analysis artifacts.

Flags:
- `--format string` - Output format: `vault|summary|prd|tasks` (default: `vault`)
- `--stdout` - Output markdown to stdout
- `--output string` - Output file path

## Architecture

### Pipeline Stages

1. **Ingest** - Fetch source material (YouTube via yt-dlp)
   - Input: URL
   - Output: `source.json` (SourceRecord)

2. **Extract** - LLM extracts structured findings
   - Input: SourceRecord
   - Output: `extraction.json` (ExtractionRecord)
   - Finds: summary, tools, workflows, opportunities, claims, quotes

3. **Lens** - LLM applies lens perspective
   - Input: ExtractionRecord + Lens definition
   - Output: `lens.json` (LensResult)
   - Produces: relevance score, ranked ideas, recommended artifacts

4. **Emit** - Generate vault note (pure template)
   - Input: SourceRecord + ExtractionRecord + LensResult
   - Output: Markdown vault note
   - No LLM call - deterministic rendering

### Design Principles

- **2 LLM calls per run:** extract → lens (emitter is pure Go template)
- **Keyless CLI:** No API keys in binary, uses stdin-mode or external LLM command
- **Model-agnostic:** Works with any LLM via `--llm-cmd`
- **Composable:** Each stage works independently

### Artifact Layout

```
artifacts/content-breakdown/2026-03-14_video-title/
├── source.json       # SourceRecord (transcript + metadata)
├── extraction.json   # ExtractionRecord (structured findings)
├── lens.json         # LensResult (ranked insights)
├── manifest.json     # ArtifactManifest (what was emitted + when)
└── note.md           # Emitted vault note
```

## Package Layout

```
breakdown/
├── main.go                      # Entry point
├── cmd/                         # Cobra CLI commands
│   ├── root.go
│   ├── version.go
│   ├── run.go                   # Happy-path orchestration
│   ├── ingest.go                # Source ingestion
│   ├── analyze.go               # Extraction + lens
│   └── emit.go                  # Artifact emission
├── internal/
│   ├── schema/record.go         # SourceRecord, ExtractionRecord, LensResult
│   ├── youtube/ingest.go        # yt-dlp wrapper + VTT/JSON3 parser
│   ├── extract/
│   │   ├── extract.go           # Extraction pass
│   │   └── prompts.go           # Extraction prompt template
│   ├── lens/
│   │   ├── lens.go              # Lens execution
│   │   └── prompts.go           # Lens prompt template
│   └── emit/vault.go            # Vault note markdown generation
├── lenses/
│   └── openclaw-product.json    # Lens definition
└── go.mod
```

## Lenses

Lenses are JSON files that define a perspective for analyzing content.

Built-in lenses:
- `openclaw-product` - material relevant to OpenClaw product development
- `personal-os` - systems and workflows for a personal operating system
- `tooling-worth-stealing` - concrete product/UX/tooling ideas worth adapting
- `founder-research` - market/product signals useful for founder decisions

Custom lenses can be placed in:
- `./lenses/<id>.json`
- `~/.openclaw/lenses/<id>.json`

## Development

```bash
# Build
go build -o breakdown .

# Test with a real video
./breakdown run "https://youtube.com/watch?v=..." --llm-cmd "claude -p" --stdout

# Run tests
go test ./...
```

## Skill Wrapper

A thin OpenClaw skill wrapper is included at:

- `skills/content-breakdown/SKILL.md`

It runs the CLI end-to-end and can optionally save the generated note into Obsidian when explicitly requested.

## Roadmap

See `ROADMAP.md` for the phased build plan.

## See Also

- Architecture review: `~/.openclaw/workspace/content-breakdown-mvp-review.md`
- Build brief: `~/.openclaw/workspace/builder-briefs/content-breakdown-phase1.md`
