# Releasing rite

`rite` follows [semver](https://semver.org). Releases are cut by tagging
`v*` on `main`; `goreleaser` publishes archives, packages, and the
Homebrew tap in one pass.

This doc is the pre-flight checklist. Work through it top to bottom; do
not skip steps. If any step fails, fix first and restart — do not paper
over.

## Pre-tag audit

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
      `rg -n 'v0\.\d+\.\d+|@v\d+\.\d+\.\d+|ubi:clintmod/rite = "v' README.md website/src/`
      Do NOT bump historical roadmap bullets in README (Phase 5 really
      shipped v0.1.0) or anything in CHANGELOG (that's history, not
      examples).
- [ ] `internal/version/version.txt` contains `v0.1.0` (the fallback
      for `go install` when ldflags are NOT injected). **Do not bump.**
      Release archives get the correct version from goreleaser's
      `-ldflags="-X internal/version.version=<tag>"` injection.
- [ ] `.goreleaser.yml` sanity: `release.draft: false`, Homebrew tap
      block names `clintmod/homebrew-tap`, and the workflow references
      the `HOMEBREW_TAP_TOKEN` secret.
- [ ] `HOMEBREW_TAP_TOKEN` secret on `clintmod/rite` is still valid (PAT
      not expired, scoped to `clintmod/homebrew-tap` only):
      `gh secret list --repo clintmod/rite`

## Tag

From a clean main checkout at the intended SHA:

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
      `v<X.Y.Z>` when run. (The `@v<X.Y.Z>` route uses ldflags so this
      should work; the `@latest` route falls back to the embedded
      `v0.1.0` from `version.txt` — expected, documented limitation.)
- [ ] Docs site rebuilt: `clintmod.github.io/rite/` renders the new
      CHANGELOG section and any updated nav.
- [ ] Smoke run on a non-trivial Ritefile: `rite --list-all` enumerates,
      at least one task runs successfully.

## If something broke

- Broken release archive: delete the GitHub release (keep the tag —
  deleting a tag you've already published breaks install scripts). Fix,
  then cut a `.Z+1` patch.
- Broken Homebrew tap: the goreleaser workflow is re-runnable from the
  GitHub Actions UI with the same tag — it will force-push the tap.
- Broken `go install`: check `internal/version/version.txt` wasn't
  manually edited and that `.goreleaser.yml`'s ldflags block points at
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
