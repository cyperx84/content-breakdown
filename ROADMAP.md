# Content Breakdown — Roadmap

Built as phased, always-working slices.

## Phase 1 — MVP Core ✅
- YouTube ingest via `yt-dlp`
- extraction pass
- lens pass
- vault note emission
- manifest emission
- thin OpenClaw skill wrapper
- real end-to-end validation

## Phase 2 — Better Outputs 🚧
- multiple emit formats from same artifacts
  - vault
  - summary
  - PRD seed
  - task list
- richer manifest metadata
- Obsidian-friendly output conventions
- tests for renderers

## Phase 3 — More Lenses
- personal-os lens
- tooling-worth-stealing lens
- founder/research lens
- content-marketing lens
- lens authoring docs

## Phase 4 — More Sources ✅
- article/webpage ingest
- local markdown/text ingest
- PDF ingest
- normalized source adapter interface

## Phase 5 — Batch + Automation ✅
- batch mode for URL lists / stdin
- parallel processing (`--parallel N`)
- skip-errors mode
- per-run summary output

## Phase 6 — Quality + Packaging ✅
- lens validation tests
- JSON extraction tests
- improved skill wrapper docs
- binary excluded from git

## Phase 7 — Deep OpenClaw Integration ✅
- skill wrapper updated for all sources/formats/lenses
- vault write instructions
- batch mode documented in skill

## Execution Rules
- keep CLI working at every phase
- ship with tests as features land
- validate each phase with at least one end-to-end run
- only stop for truly blocking issues
