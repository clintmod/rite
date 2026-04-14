# Releasing rite

`rite` uses **CalVer** for continuous releases: tag format
`v<YYYY>.<M>.<D>` with no leading zeros (Go's semver parser is strict —
`v2026.04.14` is invalid, use `v2026.4.14`). v1.0.0 stands as the last
SemVer tag and the closed contract for the initial stable release; every
release after it is CalVer.

Releases are cut by tagging `v*` on `main`; `goreleaser` publishes
archives, packages, and the Homebrew tap in one pass.

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

From a clean main checkout at the intended SHA. Replace `2026.4.14`
with today's date (no leading zeros):

```bash
git checkout main
git pull origin main
git tag -a v2026.4.14 -m "rite v2026.4.14"
git push origin v2026.4.14
```

Do **not** force-push a tag once published. If the tag points at the
wrong SHA or the release is broken, cut a new dated release (either
bump to the next day, or — if a same-day re-release is unavoidable —
append a prerelease suffix: `v2026.4.14-1`, `v2026.4.14-2`, …). Go
semver sorts `v2026.4.14-1` **before** `v2026.4.14`, so only use
prerelease suffixes when you've already planned for re-release; the
simpler path is one release per day, bumping the date field for any
re-cut.

## Post-tag verify (within ~10 minutes)

- [ ] goreleaser workflow ran and is green:
      `gh run list --repo clintmod/rite --workflow goreleaser.yml --limit 3`
- [ ] Release page populated at
      `https://github.com/clintmod/rite/releases/tag/v2026.4.14` with
      archives for darwin / linux / windows / freebsd × amd64 / arm64 /
      arm / 386 / riscv64, plus `.deb` / `.rpm` / `.apk`.
- [ ] Homebrew tap updated:
      `brew update && brew info clintmod/tap/rite` shows the new
      version.
- [ ] `brew upgrade rite` on a dev box fetches the new binary, and
      `rite --version` reports `v2026.4.14` (no `dirty`, no commit hash).
- [ ] `go install github.com/clintmod/rite/cmd/rite@v2026.4.14` reports
      `v2026.4.14` when run. (The `@v<tag>` route uses module build info
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

## CalVer bump rules

- **Tag format:** `v<YYYY>.<M>.<D>` — e.g. `v2026.4.14`,
  `v2026.11.3`. No zero-padding; Go's semver parser rejects
  `v2026.04.14` and `v2026.4.04`.
- **One release per day is the default.** If you need to cut again
  today after publishing, prefer waiting until tomorrow and bumping the
  date. Same-day re-releases are allowed via prerelease suffix `-N`
  (`v2026.4.14-1`, then `-2`, etc.). Go semver sorts these **before**
  the base version, so `-1` → `-2` → … → base isn't a meaningful
  ordering of re-cuts; treat the suffix as an escape hatch, not a
  routine.
- **Never retag.** Once a tag is pushed, the next release gets a new
  tag. Install scripts and Homebrew/mise caches pin against tag SHAs.
- **No MAJOR/MINOR/PATCH distinction.** rite's SemVer contract ended
  with v1.0.0. Breaking changes after that are still called out in the
  CHANGELOG `### Changed` / `### Removed` sections, and the SPEC
  records what's locked and what's fluid — but the tag itself carries
  no compatibility promise.

## Why CalVer

rite is a continuously-evolving developer tool with a single primary
maintainer. SemVer's value is communicating a compatibility contract
to downstream consumers — useful when you have many; overhead when
you don't. CalVer removes the MAJOR/MINOR/PATCH debate, makes
release cadence legible at a glance, and matches what this project
actually does (ship fixes as soon as they're ready). v1.0.0 stays as
the final SemVer tag so anyone who pinned against it sees a stable,
documented contract for that slice of history.
