# Project Milestones: rite

## v0.1.0 Fork + Rebrand + First Release (Shipped: 2026-04-12)

**Delivered:** Hard fork of `go-task/task` with inverted-to-Unix variable precedence, rebranded as `rite`, with a migration tool, release pipeline, Homebrew tap, and public docs site.

**Phases completed:** 0–5 (all shipped in a single day as individual waves, pre-GSD-bootstrap)

**Key accomplishments:**

- **Phase 0**: SPEC, LICENSE, NOTICE, README written — design contract for first-in-wins variable precedence locked in before any code changed.
- **Phase 1 / 1.5**: Module path → `github.com/clintmod/rite`, binary → `rite`, file discovery → `Ritefile*`, env prefix → `RITE_*`, log prefix + error strings + `rite --init` rebranded. Go package name kept as `task` to minimize churn.
- **Phase 2**: Rewrote `getVariables()` to walk first-in-wins with per-resolution dynamic-var cache, replacing upstream's last-in-wins walk.
- **Phase 3**: Fixture audit/rewrite plus the include-site var precedence fix — upstream's `Taskfile.Merge` was flattening vars from included files, breaking scoping.
- **Phase 4**: `${VAR}` shell-native preprocessor, `export: false` opt-out, vars/env unified under a single precedence model, shell env always wins.
- **Phase 5**: `rite migrate` Taskfile→Ritefile converter with 5-warning taxonomy (OVERRIDE-VAR, OVERRIDE-ENV, DOTENV-ENTRY, SECRET-VAR, SCHEMA-URL). VitePress docs site published at `clintmod.github.io/rite`. goreleaser pipeline with Homebrew tap (`clintmod/homebrew-tap`), deb/rpm/apk, and archives for darwin/linux/windows/freebsd × amd64/arm64/arm/386/riscv64.

**Stats:**

- ~90 commits from fork-start to v0.1.0
- Single-day execution (2026-04-12, 10:46 → 22:26)
- 6 phases (0, 1, 1.5, 2, 3, 4, 5 counting 1.5 as its own), 15+ waves

**Git range:** `rebrand: log prefix` (`d096a229`) → `docs: install instructions` (`82dbd415`)

**What's next:** v0.2 — docs site buildout to full taskfile.dev/docs/guide parity, JSON schema publication at `clintmod.github.io/rite/schema.json`, re-enablement of `lint-jsonschema` CI. v1.0.0 cut after that lands.

---
