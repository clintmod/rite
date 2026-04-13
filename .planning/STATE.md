# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-12)

**Core value:** Variable precedence that matches every other Unix tool — the value set closest to the user wins.
**Current focus:** Phase 6 — Docs user-guide expansion (milestone v0.2)

## Current Position

Phase: 7 of 8 (JSON schema publication) — next up
Plan: — (Phase 6 just completed; Phase 7 not yet planned)
Status: Phase 6 complete; ready to start Phase 7
Last activity: 2026-04-13 — Phase 6 docs expansion shipped (6 commits, 14 new pages, 4 expansions, sidebar restructure). FEATURE-MAP audit confirmed all 19 features exist; DOCS-18 was the only meaningful divergence.

Progress: [████████░░] 80% (8 of 10 phases complete; v0.1.0 phases collapsed)

## Performance Metrics

**Velocity:**
- Total plans completed: — (v0.1.0 pre-dates `.planning/`; phases tracked via commit messages)
- Average duration: —
- Total execution time: v0.1.0 shipped in ~12h (2026-04-12 10:46 → 22:26)

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 0–5 (v0.1.0) | pre-GSD | — | — |

**Recent Trend:**
- v0.1.0 milestone: all phases shipped in a single day
- Trend: n/a — this is the first GSD-tracked milestone

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table. Recent decisions affecting current work:

- Pre-v0.1.0: Hard fork over upstream PR; first-in-wins precedence; one-way migration only.
- v0.1.0 Phase 5: VitePress over Docusaurus for docs; `draft: false` in goreleaser for mise compatibility.
- v0.2 bootstrap: Start `.planning/` from this milestone forward — phases 0–5 stay summarized, not backfilled.
- v0.2 Phase 6 (today): Skipped GSD agent pipeline (planner/checker/verifier) for the docs phase. Cost vs. value didn't pencil out for judgment-heavy creative work with one maintainer in one session. Kept the audit-first discipline (FEATURE-MAP.md) and per-batch atomic commits with `docs(06-NN)` prefix. Pipeline stays available for future code-heavy phases (Phase 7 schema work, Phase 8 v1.0 cut).

### Pending Todos

None captured yet. Phase 7 has scope from REQUIREMENTS.md (SCHEMA-01..04) but no plan yet.

### Blockers/Concerns

- **VitePress + Shiki + Vue gotcha is broader than originally documented.** CLAUDE.md "Known sharp edges" updated in commit (next commit). Tl;dr: `${VAR}` form works everywhere; `{{.VAR}}` works in inline backticks but fails in fenced blocks containing Go-template helpers (`index`, `if`, `splitArgs ... | len`). When a feature requires showing Go-template syntax in a YAML example, fall back to "describe in prose, link to testdata fixture for runnable code."
- **Always build VitePress from `website/`, never from project root.** Building from root walks the project's `CHANGELOG.md` and `README.md`, both of which contain `{{.VAR}}` examples that will fail the same Vue gotcha. Phase 6 hit this and the workaround is `cd website && npx vitepress build`.
- **`lint-jsonschema` job was removed** in `9155ba6c` — re-enabling it is Phase 7 scope (SCHEMA-04). Do not re-add to CI until Phase 7 ships the hosted schema.

## Session Continuity

Last session: 2026-04-13
Stopped at: Phase 6 complete (6 commits, 14 new pages, 4 expansions, sidebar restructure, CLAUDE.md gotcha update). All shipped to `main`. Phase 7 (JSON schema publication) is the next logical step — REQUIREMENTS already defines SCHEMA-01..04 scope.
Resume file: None
