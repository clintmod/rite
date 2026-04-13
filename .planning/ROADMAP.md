# Roadmap: rite

## Milestones

- ✅ **v0.1.0 Fork + Rebrand + First Release** — Phases 0–5 (shipped 2026-04-12)
- 🚧 **v0.2 Docs & Schema** — Phases 6–7 (in progress)
- 📋 **v1.0 Stable** — Phase 8+ (planned)

## Phases

<details>
<summary>✅ v0.1.0 Fork + Rebrand + First Release (Phases 0–5) — SHIPPED 2026-04-12</summary>

Pre-dates `.planning/` bootstrap. Phase history tracked in commit messages (`Phase 5 wave 1`, etc.) and summarized in `MILESTONES.md` and `CLAUDE.md`. Phase-level detail:

### Phase 0: SPEC & Identity
**Goal**: Lock the design contract before any code changes.
**Delivered**: SPEC.md, LICENSE, NOTICE, README.md.
**Status**: Complete

### Phase 1: Rebrand (module path, binary, discovery)
**Goal**: Rename module to `github.com/clintmod/rite`, binary to `rite`, discover `Ritefile*`, use `RITE_*` env prefix.
**Status**: Complete (`bebe02bc`)

### Phase 1.5: Cosmetic polish
**Goal**: Log prefix, error strings, `rite --init` output.
**Status**: Complete (`d096a229`, `4600626f`)

### Phase 2: First-in-wins `getVariables()`
**Goal**: Replace upstream's last-in-wins variable walk with first-in-wins + per-resolution dynamic cache.
**Status**: Complete (`8d7ebd7f`, `ffb9838e`)

### Phase 3: Test fixture audit + include-site precedence fix
**Goal**: Rewrite fixtures for the new semantics; fix `Taskfile.Merge` flattening bug breaking include-site var scoping.
**Status**: Complete (`6183bc96` → `01c62390`)

### Phase 4: `${VAR}` preprocessor, `export: false`, vars/env unification
**Goal**: Shell-native template syntax; opt-out for secrets; single precedence model for vars and env.
**Status**: Complete (`da018dc6`, `419f2f96`, `75930421`)

### Phase 5: Migrate tool + release pipeline + docs site
**Goal**: Ship `rite migrate`, goreleaser pipeline, Homebrew tap, mise support, VitePress docs site, v0.1.0 tag.
**Status**: Complete (`f1d1d121` → `82dbd415`)

</details>

### 🚧 v0.2 Docs & Schema (In Progress)

**Milestone Goal:** Full user-guide parity with `taskfile.dev/docs/guide`, plus hosted JSON schema so editors light up.

#### Phase 6: Docs user-guide expansion ✅
**Goal**: Reach feature parity with `taskfile.dev/docs/guide` by adding the 14 missing pages and expanding the 5 thin ones. Every new page verifies `rite`'s actual behavior against the documented feature and flags divergence from upstream caused by first-in-wins semantics.
**Status**: Complete (2026-04-13)
**Requirements**: DOCS-01..19 (all complete)
**What shipped**:
  - `FEATURE-MAP.md` audit confirmed all 19 features exist in `rite`; only DOCS-18 (env precedence) diverges meaningfully.
  - 14 new pages: internal-tasks, dir, platforms, calling-tasks, cli-args, wildcards, defer, aliases, label, prompt, silent-dry-ignore, set-shopt, watch, interactive.
  - 4 expansions: cli.md (help/list/summary section), ci.md (new), precedence.md (env: block section), short-syntax.md (new).
  - Sidebar restructured from 4 groups (Start here / Features / Reference / Coming from go-task) into 5 (Start here / Tasks / Execution / Reference / Coming from go-task) with all new pages wired in.
  - CLAUDE.md updated with broader VitePress + Vue gotcha (multi-cause taxonomy).
**Plans (executed without GSD agent pipeline; tracked via commit prefix `docs(06-NN)`)**:
- [x] 06-01: Feature-map audit (FEATURE-MAP.md) — `ed7ca2d8`
- [x] 06-02: Internal, aliases, label, short-syntax pages (Batch A) — `90bc1ee9`
- [x] 06-03: Dir, platforms, defer, prompt, silent-dry-ignore (Batch B) — `e049cbfc`
- [x] 06-04: Calling-tasks, cli-args, wildcards, interactive (Batch C) — `45de2979`
- [x] 06-05: Set-shopt, watch (Batch D) — `59caca4b`
- [x] 06-06: CLI expansion, ci page, precedence env section, sidebar (Batch E) — `0ae8b616`

#### Phase 7: JSON schema publication
**Goal**: Publish `schema.json` at `clintmod.github.io/rite/schema.json`, wire editor hints into the `rite --init` template, and re-enable the `lint-jsonschema` CI job that was disabled in `9155ba6c`.
**Depends on**: Phase 6 (docs site structure stable enough to embed `public/schema.json` without collision).
**Requirements**: SCHEMA-01, SCHEMA-02, SCHEMA-03, SCHEMA-04
**Success Criteria** (what must be TRUE):
  1. `curl https://clintmod.github.io/rite/schema.json` returns a valid JSON Schema draft-07+ document.
  2. A fresh `rite --init` writes a `Ritefile.yml` whose `# yaml-language-server:` header points at the hosted schema and gives autocomplete in VS Code + Zed with zero extra config.
  3. `lint-jsonschema` runs on every push, blocking merges where `website/src/public/schema.json` is malformed or drifts from the AST.
  4. Schema covers every documented field in `SPEC.md` — especially `export: false`, `${VAR}` templating markers, and the first-in-wins precedence hints (via descriptions).
**Plans**: TBD (roadmapper will split during `/gsd:plan-phase 7`)

Plans:
- [ ] 07-01: TBD (likely: schema generation or curation from `taskfile/ast/`)
- [ ] 07-02: TBD (likely: serve via VitePress `public/`, `--init` template update, CI re-enable)

### 📋 v1.0 Stable (Planned)

**Milestone Goal:** Cut `v1.0.0` — audit experiments, drop `pre-1.0` messaging, ship verified completion scripts.

#### Phase 8: v1.0 cut (TBD)
**Goal**: Remove "pre-1.0" messaging, decide on `RITE_X_REMOTE_TASKFILES`, ship `rite`-verified completion scripts, tag `v1.0.0`.
**Requirements**: V1-01, V1-02, V1-03

## Progress

**Execution Order:**
Phases execute in numeric order: 6 → 7 → 8.

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 0. SPEC & Identity | v0.1.0 | — | Complete | 2026-04-12 |
| 1. Rebrand | v0.1.0 | — | Complete | 2026-04-12 |
| 1.5. Cosmetic polish | v0.1.0 | — | Complete | 2026-04-12 |
| 2. First-in-wins vars | v0.1.0 | — | Complete | 2026-04-12 |
| 3. Fixtures + include precedence | v0.1.0 | — | Complete | 2026-04-12 |
| 4. `${VAR}` + export + unify | v0.1.0 | — | Complete | 2026-04-12 |
| 5. Migrate + release + docs site | v0.1.0 | — | Complete | 2026-04-12 |
| 6. Docs user-guide expansion | v0.2 | 6/6 | Complete | 2026-04-13 |
| 7. JSON schema publication | v0.2 | 0/TBD | Not started | — |
| 8. v1.0 cut | v1.0 | 0/TBD | Not started | — |
