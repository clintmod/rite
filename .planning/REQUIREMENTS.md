# Requirements: rite — Milestone v0.2 Docs & Schema

**Defined:** 2026-04-12
**Core Value:** Variable precedence that matches every other Unix tool — the value set closest to the user wins.

## v0.2 Requirements

Requirements for milestone v0.2 (docs parity + schema publication). Each maps to a roadmap phase.

### Docs: Missing Guide Pages

Pages that exist on `taskfile.dev/docs/guide` and have no counterpart on `clintmod.github.io/rite`. Each page must (a) verify the feature exists in `rite` post-fork, (b) flag any divergence from upstream behavior caused by first-in-wins, (c) use `${VAR}` preprocessor style in examples.

- [ ] **DOCS-01**: Page on **internal tasks** (`internal: true`) with example and CLI exclusion behavior
- [ ] **DOCS-02**: Page on **task working directory** (`dir:`) including default behavior and relative path resolution
- [ ] **DOCS-03**: Page on **platform-specific tasks and commands** (`platforms:`) for OS/arch gating
- [ ] **DOCS-04**: Page on **calling another task** from within a task (`task:` command form) with `vars:` forwarding
- [ ] **DOCS-05**: Page on **forwarding CLI arguments** (`.CLI_ARGS` special var) with `--` passthrough semantics
- [ ] **DOCS-06**: Page on **wildcard task names** (`*` matching) with captured-wildcard access
- [ ] **DOCS-07**: Page on **`defer:` cleanup** that runs after task completion (success and failure paths)
- [ ] **DOCS-08**: Page on **task aliases** (alternate invocation names)
- [ ] **DOCS-09**: Page on **`label:` display override** for custom task names in output
- [ ] **DOCS-10**: Page on **`prompt:` warning prompts** for destructive/risky tasks
- [ ] **DOCS-11**: Page covering the **silent / dry-run / ignore-errors** trio (`silent:`, `--dry`, `ignore_error:`)
- [ ] **DOCS-12**: Page on **`set:` and `shopt:`** for controlling shell options per task or Ritefile
- [ ] **DOCS-13**: Page on **watch mode** (`--watch`) with source-change detection
- [ ] **DOCS-14**: Page on **interactive CLI applications** (`interactive: true` for TUIs/REPLs)

### Docs: Expansions

Pages already present but thin. Each gets expanded to match upstream depth while staying honest about what `rite` does differently.

- [ ] **DOCS-15**: Expand `cli.md` with full **help / `--list` / `--list-all` / `--summary`** coverage
- [ ] **DOCS-16**: Add **CI integration** notes (colored output, GitHub Actions annotations)
- [ ] **DOCS-17**: Document **short task syntax** (single-string shorthand for trivial tasks)
- [ ] **DOCS-18**: Expand `precedence.md` with an explicit **`env:` block section** — currently conflated with vars
- [ ] **DOCS-19**: Cross-link guide — every new page linked from `getting-started.md` and the VitePress sidebar config

### Schema Publication

- [ ] **SCHEMA-01**: Generate / curate `schema.json` from the `rite` taskfile AST (drop upstream's removed-pre-fork schema; build one that matches current SPEC)
- [ ] **SCHEMA-02**: Ship `schema.json` via the VitePress `public/` dir so it's served at `clintmod.github.io/rite/schema.json`
- [ ] **SCHEMA-03**: Add `# yaml-language-server: $schema=…` hint to the `rite --init` template so new Ritefiles get editor autocomplete
- [ ] **SCHEMA-04**: Re-enable the `lint-jsonschema` CI job against the hosted schema (deleted in `9155ba6c`; needs updated path + a smoke fixture)

## v1.0 Requirements (Deferred)

Tracked but not in current milestone. Promotes to milestone v1.0 after v0.2 ships.

### Cut v1.0

- **V1-01**: Cut `v1.0.0` tag after docs + schema validate; update README/status/roadmap messaging from "pre-1.0" to "stable"
- **V1-02**: Audit `RITE_X_REMOTE_TASKFILES` experiment — decide graduate, keep experimental, or remove
- **V1-03**: Populate and ship completion scripts verified for `rite` (currently still shipping upstream `task` completions bound to `rite` name)

## Out of Scope

Explicitly excluded from v0.2. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Compat shim for `go-task` Taskfiles | SPEC is an intentional semantic break; migration is one-way by design |
| Port of upstream's Docusaurus site | VitePress site already shipped; re-porting buys nothing |
| Windows-native `${VAR}` semantics | Preprocessor targets POSIX; Windows works but isn't the design target |
| Graduating `RITE_X_REMOTE_TASKFILES` | Needs separate audit; deferred to v1.0 requirement V1-02 |
| Mobile / IDE plugins | Not core value; JSON schema gives editors the primary hook they need |

## Traceability

Which phases cover which requirements. Updated by roadmapper during `/gsd:plan-phase`.

| Requirement | Phase | Status |
|-------------|-------|--------|
| DOCS-01 | Phase 6 | Pending |
| DOCS-02 | Phase 6 | Pending |
| DOCS-03 | Phase 6 | Pending |
| DOCS-04 | Phase 6 | Pending |
| DOCS-05 | Phase 6 | Pending |
| DOCS-06 | Phase 6 | Pending |
| DOCS-07 | Phase 6 | Pending |
| DOCS-08 | Phase 6 | Pending |
| DOCS-09 | Phase 6 | Pending |
| DOCS-10 | Phase 6 | Pending |
| DOCS-11 | Phase 6 | Pending |
| DOCS-12 | Phase 6 | Pending |
| DOCS-13 | Phase 6 | Pending |
| DOCS-14 | Phase 6 | Pending |
| DOCS-15 | Phase 6 | Pending |
| DOCS-16 | Phase 6 | Pending |
| DOCS-17 | Phase 6 | Pending |
| DOCS-18 | Phase 6 | Pending |
| DOCS-19 | Phase 6 | Pending |
| SCHEMA-01 | Phase 7 | Pending |
| SCHEMA-02 | Phase 7 | Pending |
| SCHEMA-03 | Phase 7 | Pending |
| SCHEMA-04 | Phase 7 | Pending |

**Coverage:**
- v0.2 requirements: 23 total
- Mapped to phases: 23
- Unmapped: 0 ✓

---
*Requirements defined: 2026-04-12*
*Last updated: 2026-04-12 after initial milestone v0.2 definition*
