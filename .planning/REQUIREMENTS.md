# Requirements: rite — Milestone v0.2 Docs & Schema

**Defined:** 2026-04-12
**Core Value:** Variable precedence that matches every other Unix tool — the value set closest to the user wins.

## v0.2 Requirements

Requirements for milestone v0.2 (docs parity + schema publication). Each maps to a roadmap phase.

### Docs: Missing Guide Pages

Pages that exist on `taskfile.dev/docs/guide` and have no counterpart on `clintmod.github.io/rite`. Each page must (a) verify the feature exists in `rite` post-fork, (b) flag any divergence from upstream behavior caused by first-in-wins, (c) use `${VAR}` preprocessor style in examples.

- [x] **DOCS-01**: Page on **internal tasks** (`internal: true`) — `website/src/internal-tasks.md`
- [x] **DOCS-02**: Page on **task working directory** (`dir:`) — `website/src/dir.md`
- [x] **DOCS-03**: Page on **platform-specific tasks and commands** (`platforms:`) — `website/src/platforms.md`
- [x] **DOCS-04**: Page on **calling another task** — `website/src/calling-tasks.md`
- [x] **DOCS-05**: Page on **forwarding CLI arguments** (`.CLI_ARGS`) — `website/src/cli-args.md`
- [x] **DOCS-06**: Page on **wildcard task names** — `website/src/wildcards.md`
- [x] **DOCS-07**: Page on **`defer:` cleanup** — `website/src/defer.md`
- [x] **DOCS-08**: Page on **task aliases** — `website/src/aliases.md`
- [x] **DOCS-09**: Page on **`label:` display override** — `website/src/label.md`
- [x] **DOCS-10**: Page on **`prompt:` warning prompts** — `website/src/prompt.md`
- [x] **DOCS-11**: Page covering the **silent / dry-run / ignore-errors** trio — `website/src/silent-dry-ignore.md`
- [x] **DOCS-12**: Page on **`set:` and `shopt:`** — `website/src/set-shopt.md`
- [x] **DOCS-13**: Page on **watch mode** — `website/src/watch.md`
- [x] **DOCS-14**: Page on **interactive CLI applications** — `website/src/interactive.md`

### Docs: Expansions

Pages already present but thin. Each gets expanded to match upstream depth while staying honest about what `rite` does differently.

- [x] **DOCS-15**: Expanded `cli.md` with **help / `--list` / `--list-all` / `--summary`** section
- [x] **DOCS-16**: Added **CI integration** as its own page (`website/src/ci.md`) — NO_COLOR, GitHub Actions groups, CI install, exit codes
- [x] **DOCS-17**: Documented **short task syntax** as its own page (`website/src/short-syntax.md`)
- [x] **DOCS-18**: Expanded `precedence.md` with explicit **`env:` block section** including dotenv tier 4 and divergence-from-go-task callout
- [x] **DOCS-19**: Sidebar in `website/.vitepress/config.ts` restructured into 5 groups (Start here, Tasks, Execution, Reference, Coming from go-task) with all 14 new pages + ci page wired in

### Schema Publication

- [x] **SCHEMA-01**: Generated via `cmd/gen-schema/` reflecting on `taskfile/ast/*`. Code-gen approach (option b2): yaml struct tags + invopop/jsonschema + custom Mapper for polymorphic types.
- [x] **SCHEMA-02**: Served at `clintmod.github.io/rite/schema.json` via `website/src/public/schema.json`.
- [x] **SCHEMA-03**: `internal/task/templates/default.yml` now points `yaml-language-server: $schema` at the hosted URL.
- [x] **SCHEMA-04**: `lint-jsonschema` CI job re-enabled. Regenerates + diffs against the committed schema (catches stale schemas), metaschema-validates the schema itself, and validates every `testdata/**/Ritefile.yml` fixture against the schema.

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
| DOCS-01 | Phase 6 | Complete |
| DOCS-02 | Phase 6 | Complete |
| DOCS-03 | Phase 6 | Complete |
| DOCS-04 | Phase 6 | Complete |
| DOCS-05 | Phase 6 | Complete |
| DOCS-06 | Phase 6 | Complete |
| DOCS-07 | Phase 6 | Complete |
| DOCS-08 | Phase 6 | Complete |
| DOCS-09 | Phase 6 | Complete |
| DOCS-10 | Phase 6 | Complete |
| DOCS-11 | Phase 6 | Complete |
| DOCS-12 | Phase 6 | Complete |
| DOCS-13 | Phase 6 | Complete |
| DOCS-14 | Phase 6 | Complete |
| DOCS-15 | Phase 6 | Complete |
| DOCS-16 | Phase 6 | Complete |
| DOCS-17 | Phase 6 | Complete |
| DOCS-18 | Phase 6 | Complete |
| DOCS-19 | Phase 6 | Complete |
| SCHEMA-01 | Phase 7 | Complete |
| SCHEMA-02 | Phase 7 | Complete |
| SCHEMA-03 | Phase 7 | Complete |
| SCHEMA-04 | Phase 7 | Complete |

**Coverage:**
- v0.2 requirements: 23 total
- Mapped to phases: 23
- Unmapped: 0 ✓

---
*Requirements defined: 2026-04-12*
*Last updated: 2026-04-12 after initial milestone v0.2 definition*
