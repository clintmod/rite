# Releasing rite

`rite` follows [semver](https://semver.org). Releases are cut by tagging
`v*` on `main`; `goreleaser` publishes archives, packages, and the
Homebrew tap in one pass.

The release pipeline is codified as three Ritefile tasks. Run them in
order; each one validates its own preconditions and refuses cleanly on
mismatch (no partial state). If any task fails, fix the underlying
issue and re-run — do not paper over.

## TL;DR — three commands

```bash
rite release:prepare VERSION=X.Y.Z   # opens staging PR, bumps everything
# ...review CHANGELOG blurb, get CI green, squash-merge the PR...
git checkout main && git pull origin main
rite release:tag VERSION=X.Y.Z       # tags merged commit, pushes tag
rite release:verify VERSION=X.Y.Z    # polls workflow + asserts artifacts
```

`VERSION` is bare semver (no `v` prefix). Each task regex-validates it.

The detailed checklist below documents what each task does internally —
useful for debugging, partial recovery, and audit. The tasks themselves
are the source of truth; this doc trails them.

## What each task does

### `release:prepare VERSION=X.Y.Z`

Codifies the **Pre-tag audit** checklist. Idempotent at the branch
level — re-running while `release/v$VERSION` already exists refuses
cleanly.

1. Validates `VERSION` is `\d+\.\d+\.\d+`.
2. Asserts current branch is `main`, tree clean, local matches
   `origin/main`, and neither `release/v$VERSION` (branch) nor
   `v$VERSION` (tag) already exists locally or on origin.
3. Runs `rite test` and `rite lint` locally. Hard-fails on any failure.
4. Asserts the most recent push-event run on `origin/main` is
   `completed/success`.
5. Reads the previous version from `internal/version/version.txt` and
   bumps:
   - `internal/version/version.txt` → `vX.Y.Z`
   - `CHANGELOG.md` — inserts `## [X.Y.Z] - YYYY-MM-DD` immediately
     after `## [Unreleased]` (preserving the `[Unreleased]` header so
     subsequent PRs always have a place to land entries).
   - `README.md` — only the install-example pins (`**Status: v$OLD
     shipped.**`, the `~/bin v$OLD` pin example, the `ubi:` line). The
     historical roadmap bullets (`- [x] **v$OLD:**`) are deliberately
     not bumped — they record what shipped when, not what's current.
   - `website/src/{getting-started,index,ci,special-vars}.md` — full
     `s/$OLD/$NEW/g` (these files have no historical-roadmap concern).
6. Cuts a `release/v$VERSION` branch, commits the bump as
   `release: stage v$VERSION`, pushes, and opens a draft PR via `gh`.

The release-summary blurb (the one-line description that goes under
the new `## [X.Y.Z]` header in `CHANGELOG.md`) is **left blank for the
human to fill in**, on purpose — it's the only piece of release prose
that isn't mechanically derivable.

### `release:tag VERSION=X.Y.Z`

Codifies the **Tag** section. **Strict** — refuses if `v$VERSION`
already exists anywhere; never force-pushes.

1. Validates `VERSION` regex, branch is `main`, tree clean, local
   matches `origin/main`.
2. Asserts `v$VERSION` does not exist locally or on origin.
3. Asserts the merged staging changes actually landed on main:
   `internal/version/version.txt` reads `v$VERSION`, and `CHANGELOG.md`
   has a dated `## [X.Y.Z] - YYYY-MM-DD` section. Refuses if either is
   missing — that means the staging PR didn't merge or wasn't run.
4. `git tag -a v$VERSION -m "rite v$VERSION"` and pushes the tag,
   which triggers the goreleaser workflow.

### `release:verify VERSION=X.Y.Z`

Codifies the **Post-tag verify** section. Read-only, safe to re-run.

1. Polls `gh run list --workflow release.yml` for a run on the matching
   tag, up to a 10-minute deadline. Reports progress on every poll.
   Fails on workflow timeout, failure, or cancellation.
2. Asserts the GitHub release page exists for `v$VERSION` and has at
   least 10 assets (archives + checksums + packages).
3. Best-effort check that the Homebrew tap formula references the new
   version. Warns (does not fail) on mismatch — the tap bump occasionally
   lags the goreleaser run.
4. Prints the residual manual smoke checks (`brew upgrade rite`,
   `go install …@v$VERSION`, docs site rebuild) — those depend on a
   different machine or external state and aren't worth automating.

## Pre-tag audit (manual checklist — what `release:prepare` automates)

- [ ] Main branch CI fully green (most recent push on `main` shows
      Test, Lint, Coexistence, Docs, `lint-jsonschema`, Race all SUCCESS
      or SKIPPED):
      `gh run list --repo clintmod/rite --branch main --limit 5`
- [ ] `rite test` green locally on your dev machine.
- [ ] `rite lint` green locally.
- [ ] `rite verify:coexistence` green locally (dockerized dual-install smoke).
- [ ] `rite goreleaser:test` builds archives cleanly without publishing
      (dry-run / `--snapshot --clean`).
- [ ] No open issues labeled `1.0-blocker` (or whatever milestone label
      gates this release):
      `gh issue list --repo clintmod/rite --state open --label <milestone>`
- [ ] `CHANGELOG.md` has a dated `## [<version>] - YYYY-MM-DD` section
      (not `TBD`), and it names every meaningful PR merged since the
      previous tag. Cross-check against
      `git log <prev-tag>..HEAD --oneline`.
- [ ] `README.md` Roadmap section marks this version's row checked and
      names what comes next.
- [ ] `SPEC.md` describes current code behavior — walk the §Variable
      Precedence table, §File Format constraints, and §Non-goals section
      and diff against what the code actually does.
- [ ] `website/src/migration.md` and `website/src/cli.md` examples use
      current CLI surface (`rite --migrate <path>`).
- [ ] **Install examples reference the new tag.** Grep the repo for the
      previous version and bump each hit to the version you're about to
      cut. Typical places: `README.md` (install.sh pin, `ubi:` mise
      line), `website/src/` (`getting-started.md`, `index.md`, `ci.md`
      — both `go install @v*` and `install.sh … v*` lines).
      `rg -n '@v\d+\.\d+\.\d+|ubi:clintmod/rite = "v' README.md website/src/`
      Do NOT bump historical roadmap bullets in README (Phase 5 really
      shipped v0.1.0, and v1.0.0 closed the SemVer contract) or anything
      in CHANGELOG (that's history, not examples).
- [ ] **Bump `internal/version/version.txt` to match the tag you're
      about to cut.** Previously we left this pinned — that made
      `go install` from a local checkout (and any build that doesn't
      go through goreleaser's ldflags injection) report a stale
      version. Now `version.txt` tracks the last shipped tag so
      local-build fallbacks stay accurate. Release archives still get
      the correct version from goreleaser's
      `-ldflags="-X internal/version.version=<tag>"` injection — this
      bump is purely for the local-build and `go install @latest`
      paths.
- [ ] `.goreleaser.yml` sanity: `release.draft: false`, Homebrew tap
      block names `clintmod/homebrew-tap`, and the workflow references
      the `HOMEBREW_TAP_TOKEN` secret.
- [ ] `HOMEBREW_TAP_TOKEN` secret on `clintmod/rite` is still valid (PAT
      not expired, scoped to `clintmod/homebrew-tap` only):
      `gh secret list --repo clintmod/rite`

## Tag

From a clean main checkout at the intended SHA. Replace `<X.Y.Z>`
with the semver you're cutting:

```bash
git checkout main
git pull origin main
git tag -a v<X.Y.Z> -m "rite v<X.Y.Z>"
git push origin v<X.Y.Z>
```

Do **not** force-push a tag once published. If the tag points at the
wrong SHA, cut a patch release instead.

## Post-tag verify (within ~10 minutes)

- [ ] goreleaser workflow ran and is green:
      `gh run list --repo clintmod/rite --workflow goreleaser.yml --limit 3`
- [ ] Release page populated at
      `https://github.com/clintmod/rite/releases/tag/v<X.Y.Z>` with
      archives for darwin / linux / windows / freebsd × amd64 / arm64 /
      arm / 386 / riscv64, plus `.deb` / `.rpm` / `.apk`.
- [ ] Homebrew tap updated:
      `brew update && brew info clintmod/tap/rite` shows the new
      version.
- [ ] `brew upgrade rite` on a dev box fetches the new binary, and
      `rite --version` reports `v<X.Y.Z>` (no `dirty`, no commit hash).
- [ ] `go install github.com/clintmod/rite/cmd/rite@v<X.Y.Z>` reports
      `v<X.Y.Z>` when run. (The `@v<tag>` route uses module build info
      via #92; the `@latest` and local-checkout routes fall back to the
      embedded `version.txt` — which you just bumped to match the tag.)
- [ ] Docs site rebuilt: `clintmod.github.io/rite/` renders the new
      CHANGELOG section and any updated nav.
- [ ] Smoke run on a non-trivial Ritefile: `rite --list-all` enumerates,
      at least one task runs successfully.

## If something broke

- Broken release archive: delete the GitHub release (keep the tag —
  deleting a tag you've already published breaks install scripts). Fix,
  then cut a new dated release (next day, or same-day `-1` prerelease
  suffix if unavoidable).
- Broken Homebrew tap: the goreleaser workflow is re-runnable from the
  GitHub Actions UI with the same tag — it will force-push the tap.
- Broken `go install`: check `internal/version/version.txt` matches the
  current tag (the pre-tag audit should have bumped it) and that
  `.goreleaser.yml`'s ldflags block points at
  `github.com/clintmod/rite/internal/version.version`.

## When to bump MAJOR vs MINOR vs PATCH

- **MAJOR (`X+1.0.0`)** — any change to: exit code numbers, CLI flag
  meanings, file-discovery order, the variable-precedence tiers in
  SPEC, or public API identifiers under the `rite` module path.
  Rename-only changes to identifiers still count.
- **MINOR (`X.Y+1.0`)** — new CLI flags, new migrate warning classes,
  new special vars, added SPEC clauses that don't invalidate existing
  Ritefiles.
- **PATCH (`X.Y.Z+1`)** — bug fixes, doc updates, dependency bumps,
  internal refactors with no observable behavior change.

When in doubt, err toward the larger bump. A spurious major release
costs a CHANGELOG paragraph; a missed major release breaks users who
trust the semver contract.

**Why SemVer and not CalVer.** We briefly tried CalVer (`v<YYYY>.<M>.<D>`)
and hit Go's semantic import versioning rule: any major version ≥ 2
requires the module path to include a `/vN` suffix. A date-as-major
tag like `v2026.4.14` would force `github.com/clintmod/rite/v2026/...`
imports and yearly path rotations. SemVer stays under major 1 or 2
and avoids the impedance mismatch entirely.
