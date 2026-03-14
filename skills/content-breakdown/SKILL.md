---
name: content-breakdown
description: Break down a YouTube video, article, or local file into structured findings, lens-ranked ideas, and output notes using the breakdown CLI. Use when the user asks to analyze, break down, summarize into actionable notes, or turn content into a vault note, PRD, or task list.
---

# Content Breakdown

Use this skill when the user wants **content turned into structured notes**.

## Supported sources

- YouTube URLs
- Article / webpage URLs
- Local files (`.md`, `.txt`, `.pdf`)

## Usage

### Single source

```bash
cd ~/github/content-breakdown
go run . run "<url-or-file>" --stdout --llm-cmd "claude --print --permission-mode bypassPermissions"
```

### With specific lens and format

```bash
go run . run "<url>" --lens personal-os --format prd --stdout --llm-cmd "claude --print --permission-mode bypassPermissions"
```

### Batch mode

```bash
go run . batch urls.txt --llm-cmd "claude --print --permission-mode bypassPermissions" --skip-errors
```

## Available lenses

- `openclaw-product` (default)
- `personal-os`
- `tooling-worth-stealing`
- `founder-research`

## Available formats

- `vault` (default) — Obsidian-ready full note
- `summary` — executive summary
- `prd` — PRD seed document
- `tasks` — task list with checkboxes

## Artifacts

Written to `./artifacts/content-breakdown/<slug>/`:
- `source.json`, `extraction.json`, `lens.json`, `manifest.json`
- Output note (e.g. `note.md`, `prd.md`, `tasks.md`)

## Vault write option

If the user explicitly wants the note saved to Obsidian:

```bash
NOTE_PATH="inbox/$(date +%F)-content-breakdown.md"
obsidian-cli create "$NOTE_PATH" --overwrite --content-file /tmp/breakdown-note.md
```

Only write to vault when explicitly asked.

## Response style

1. Short summary of what the content was about
2. Top ranked ideas (with scores)
3. Where artifacts were written
4. Whether note was saved to Obsidian (if applicable)

## Requirements

- `yt-dlp` on PATH (for YouTube)
- `pdftotext` on PATH (for PDFs — `brew install poppler`)
- `claude` on PATH (for `--llm-cmd`)
- This skill is intentionally thin; the CLI owns the workflow
