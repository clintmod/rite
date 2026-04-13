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

#### Phase 6: Docs user-guide expansion
**Goal**: Reach feature parity with `taskfile.dev/docs/guide` by adding the 14 missing pages and expanding the 5 thin ones. Every new page verifies `rite`'s actual behavior against the documented feature and flags divergence from upstream caused by first-in-wins semantics.
**Depends on**: v0.1.0 shipped (docs site live).
**Requirements**: DOCS-01, DOCS-02, DOCS-03, DOCS-04, DOCS-05, DOCS-06, DOCS-07, DOCS-08, DOCS-09, DOCS-10, DOCS-11, DOCS-12, DOCS-13, DOCS-14, DOCS-15, DOCS-16, DOCS-17, DOCS-18, DOCS-19
**Success Criteria** (what must be TRUE):
  1. Every feature on taskfile.dev/docs/guide has a corresponding page on clintmod.github.io/rite (or an explicit Out-of-Scope note if `rite` removed the feature).
  2. Each new page includes at least one runnable Ritefile example using `${VAR}` preprocessor syntax.
  3. Any divergence from upstream behavior (first-in-wins effects, removed features, renamed flags) is called out in a "Differences from go-task" callout on the relevant page.
  4. The VitePress sidebar exposes every page under a "Guide" group.
  5. `rite lint` and `rite test` remain green; docs build (`pages.yml`) succeeds on the phase-closing commit.
**Plans**: TBD (roadmapper will split during `/gsd:plan-phase 6`)

Plans:
- [ ] 06-01: TBD (likely: audit `rite` feature matrix against taskfile.dev guide, write a behavior-map table before any page gets drafted)
- [ ] 06-02: TBD (likely: missing guide pages — batch 1 of 2)
- [ ] 06-03: TBD (likely: missing guide pages — batch 2 of 2)
- [ ] 06-04: TBD (likely: expansions to existing thin pages + sidebar/navigation update)

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
| 6. Docs user-guide expansion | v0.2 | 0/TBD | Not started | — |
| 7. JSON schema publication | v0.2 | 0/TBD | Not started | — |
| 8. v1.0 cut | v1.0 | 0/TBD | Not started | — |
