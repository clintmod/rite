# rite

## What This Is

`rite` is a task runner in the space of `make`, `just`, and `go-task` — tasks declared in YAML, run with dependency resolution, parameters, and shell invocation. What makes it different: variable precedence follows 50 years of Unix convention (first-in-wins — shell env > CLI > Ritefile defaults), not `go-task`'s inversion where task-scope `vars:` override the caller. `rite` is a hard fork of `go-task/task` with no intention of merging back.

## Core Value

**Variable precedence that matches every other Unix tool.** The value set closest to the user wins. Shell env beats CLI args beats `Ritefile` defaults. Task-scope `vars:` are defaults only. If this behavior regresses, nothing else about `rite` matters.

## Requirements

### Validated

<!-- Shipped in v0.1.0 (2026-04-12). -->

- ✓ **Repo identity, module path, binary name** — v0.1.0 Phase 0/1 (`bebe02bc`)
- ✓ **File discovery (Ritefile / Ritefile.yml / .dist variants), `RITE_*` env prefix, `--init`** — v0.1.0 Phase 1 / 1.5 (`bebe02bc`, `d096a229`, `4600626f`)
- ✓ **First-in-wins `getVariables()` with per-resolution dynamic-var cache** — v0.1.0 Phase 2 (`8d7ebd7f`, `ffb9838e`)
- ✓ **Include-site var precedence fix (`Taskfile.Merge` flattening bug)** — v0.1.0 Phase 3 (`6183bc96` through `01c62390`)
- ✓ **`${VAR}` shell-native preprocessor** — v0.1.0 Phase 4 (`da018dc6`)
- ✓ **`export: false` opt-out for vars that shouldn't leak to the cmd environ** — v0.1.0 Phase 4 (`419f2f96`)
- ✓ **vars/env unified; shell env always wins** — v0.1.0 Phase 4 (`75930421`)
- ✓ **`rite migrate` Taskfile → Ritefile converter with 5-warning taxonomy** — v0.1.0 Phase 5 (`f1d1d121`)
- ✓ **Release pipeline: goreleaser, Homebrew tap (`clintmod/homebrew-tap`), mise (ubi + go backends)** — v0.1.0 Phase 5 (`338871e9`, `82dbd415`)
- ✓ **Docs site at `clintmod.github.io/rite`** (VitePress on GitHub Pages) — v0.1.0 Phase 5 (`ab5a78a7`, `cf0af5fa`)

### Active

<!-- Current scope for milestone v0.2 (docs + schema). -->

- [ ] **DOCS-01** — Full user-guide parity with `taskfile.dev/docs/guide` (12 missing pages, 5 expansions)
- [ ] **SCHEMA-01** — Publish JSON schema at `clintmod.github.io/rite/schema.json` so editor hints work
- [ ] **SCHEMA-02** — Re-enable the `lint-jsonschema` CI job once the schema is hosted
- [ ] **V1-01** — Cut v1.0.0 after docs and schema land (reserved for next milestone)

### Out of Scope

- **Backwards-compatibility shim with `go-task`** — the SPEC is an intentional semantic break; `rite migrate` converts one-way. Re-adding compat would dilute the core value.
- **Merging `rite` back into `go-task/task`** — upstream has publicly committed to keeping the inversion ([#2035](https://github.com/go-task/task/issues/2035)). We can cherry-pick non-variable-system changes; we cannot merge.
- **Wholesale upstream merges** — `compiler.go`, `variables.go`, `internal/templater/`, `internal/env/`, `taskfile/`, and `taskfile/ast/vars*.go` are ours. Individual cherry-picks only, with `-x` to record provenance.
- **Windows as a first-class shell environment for the `${VAR}` preprocessor** — preprocessor targets POSIX semantics; Windows works but isn't the design target.
- **Remote-taskfile experiment productionization** — `RITE_X_REMOTE_TASKFILES=1` inherits unaudited upstream code paths; remains behind an experiment flag until audited.

## Context

- **Upstream relationship.** `rite` forked from `go-task/task` on 2026-04-12. The git history of upstream is preserved under MIT attribution. `upstream` remote is kept for security/platform cherry-picks; `origin` is `clintmod/rite`.
- **Package identifier is still `task`.** Module path renamed to `github.com/clintmod/rite` in Phase 1, but the Go package name stayed `task` to minimize diff. Several files use `task "github.com/clintmod/rite"` as the import alias. goimports/golangci-lint enforce this — don't remove the alias without renaming the package globally.
- **`go install` reports the wrong version.** goreleaser's ldflags inject `internal/version.version={{.Version}}` for release builds. A bare `go install github.com/clintmod/rite/cmd/rite@v0.1.0` embeds the fallback constant `3.49.1`. Brew, ubi, and the release archives all report the correct version — this is a `go install`-only quirk.
- **VitePress + Shiki + Vue bug.** Multiple `{{…}}` template expressions on one line inside YAML fenced blocks break the Vue SFC compiler with a confusing position-mismatched error. Workaround: use `${VAR}` form in examples (SPEC-preferred anyway); for inline prose, wrap in `<span v-pre>` ``{{.VAR}}`` `</span>`.
- **Older mise + `ubi:` backend.** mise < 2026.4 strips the `v` prefix when resolving `ubi:clintmod/rite`; users on older versions should use the `go:` backend fallback (documented in `getting-started.md`). CI should use mise ≥ 2026.4.
- **Obsidian vault is the master source for SPEC and decisions.** `SPEC.md` and `decisions/` live at `~/Dropbox/brain/10-projects/rite/`. When an ADR conflicts with current code, code wins and the ADR gets updated.

## Constraints

- **Tech stack**: Go 1.25+ — inherited from upstream, no compelling reason to change. CI matrix is Go 1.25 + 1.26 × {ubuntu, macos, windows}.
- **Distribution**: Homebrew (via tap), mise (ubi + go backends), goreleaser (release archives + deb/rpm/apk), `go install`. Do not add package ecosystems without a path to automation — manual maintenance is the trap we're avoiding.
- **SPEC before code**: behavioral changes touching variable semantics require a SPEC update first or in the same commit. Disagreement between CLAUDE.md and SPEC.md is resolved in favor of SPEC.
- **No partial rebrands**: any rename/restructure must land atomically; the tree is always buildable on `main`.
- **Upstream integration discipline**: every cherry-pick uses `git cherry-pick -x` and notes why we want it; files in the "ours" list above never get upstream changes pulled in.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Hard fork rather than PR upstream | Upstream committed to keeping inversion in their redesign ([#2035](https://github.com/go-task/task/issues/2035)) | ✓ Good — v0.1.0 shipped without upstream friction |
| First-in-wins variable precedence | Matches every other Unix tool; eliminates ~decade of upstream bugs traced to inversion ([#2034](https://github.com/go-task/task/issues/2034)) | ✓ Good — SPEC stable, migrate tool surfaces divergences cleanly |
| `${VAR}` preprocessor as primary template syntax | Shell-native; Go-template is harder to reason about for mixed vars/env | ✓ Good — docs and migrate tool both lean on it |
| Auto-export vars to cmd environ with `export: false` opt-out | Matches shell/Makefile mental model; secrets opt out explicitly | ✓ Good — CLI_* special vars already use it |
| One-way migration only, no compat shim | Keeps SPEC pure; compat would require maintaining both models | ✓ Good — migrate tool is small and the 5-warning taxonomy is teachable |
| VitePress for docs (vs. upstream's Docusaurus) | Smaller dep tree, simpler Pages deploy, upstream's Docusaurus depended on pro goreleaser | ✓ Good — site is live at `clintmod.github.io/rite` |
| Release with `draft: false` in goreleaser | Draft releases were invisible to mise's ubi/github backends until manually promoted | ✓ Good — v0.1.0 was reachable by mise immediately |
| Go package name stays `task` post-rebrand | Minimizes diff against upstream; rename would touch every file without semantic benefit | — Pending — revisit if we diverge enough that upstream cherry-picks become rare |
| Phase history tracked in commit messages, not `.planning/` | Phases 0-5 shipped in a single day; GSD overhead wasn't warranted | — Pending — starting `.planning/` from v0.2 forward (this milestone) |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

## Current Milestone: v0.2 Docs & Schema

**Goal:** Bring the docs site to full feature parity with `taskfile.dev/docs/guide` and publish the JSON schema so editor hints work.

**Target features:**
- Add the ~12 user-guide pages missing from the current site (internal tasks, `dir:`, `platforms:`, `task:` calling, `.CLI_ARGS`, wildcards, `defer:`, aliases, `label:`, `prompt:`, silent/dry-run/ignore-errors, `set`/`shopt`, watch, interactive).
- Expand 5 thin pages (help/list-all, summary, CI integration, short task syntax, `env:` in precedence).
- Host `schema.json` at `clintmod.github.io/rite/schema.json` and re-enable the `lint-jsonschema` CI job.
- Audit `rite` behavior against each documented feature before writing the page (some upstream features may not exist in `rite` post-fork, or may work differently under first-in-wins semantics).

---
*Last updated: 2026-04-12 after initial `.planning/` bootstrap following v0.1.0 ship*
