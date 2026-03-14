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

## Phase 4 — More Sources
- article/webpage ingest
- local markdown/text ingest
- PDF ingest
- normalized source adapter interface

## Phase 5 — Batch + Automation
- batch mode for URL lists
- resumable runs
- cron-friendly commands
- artifact indexing/search

## Phase 6 — Quality + Packaging
- fixture corpus
- regression tests on real captured artifacts
- release workflow
- install/distribution polish

## Phase 7 — Deep OpenClaw Integration
- better skill UX
- optional Obsidian auto-save flow
- project-aware artifact routing
- higher-level commands for recurring workflows

## Execution Rules
- keep CLI working at every phase
- ship with tests as features land
- validate each phase with at least one end-to-end run
- only stop for truly blocking issues
