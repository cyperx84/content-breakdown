---
name: content-breakdown
description: Break down a YouTube video into structured findings, lens-ranked ideas, and an Obsidian-ready vault note using the breakdown CLI. Use when the user asks to analyze, break down, summarize into actionable notes, or turn a YouTube video into a vault note.
---

# Content Breakdown

Use this skill when the user wants a **YouTube video turned into structured notes**.

## What it does

Runs the `breakdown` CLI end-to-end:

```bash
go run . run "<youtube-url>" --stdout --llm-cmd "claude --print --permission-mode bypassPermissions"
```

Artifacts are written to:

```bash
./artifacts/content-breakdown/<slug>/
```

Outputs:
- `source.json`
- `extraction.json`
- `lens.json`
- `manifest.json`
- `note.md` (when not using `--stdout`)

## Default lens

Use:
- `openclaw-product`

Unless the user asks for a different lens.

## Vault write option

If the user explicitly wants the note saved into Obsidian, write it with `obsidian-cli`.

Example pattern:

```bash
NOTE_PATH="inbox/$(date +%F)-content-breakdown.md"
obsidian-cli create "$NOTE_PATH" --overwrite --content-file /tmp/breakdown-note.md
```

Before writing to the vault, make sure the user asked for it.

## Response style

When replying to the user:
1. Give a short summary of what the video was about
2. List the top ranked ideas
3. Mention where artifacts were written
4. If relevant, say whether the note was also saved to Obsidian

## Notes

- Requires `yt-dlp` on PATH
- Requires `claude` on PATH for `--llm-cmd`
- This skill is intentionally thin; the CLI owns the workflow
