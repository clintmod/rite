# CI integration

`rite` runs unmodified in any CI environment that has a Go binary or a downloadable release archive available. A few flags and conventions make the experience nicer.

## Output

By default rite emits ANSI color in its `[task]` log prefixes. CI runners that don't render color end up with escape codes in their logs.

To disable color:

```sh
NO_COLOR=1 rite build
RITE_COLOR_RESET=1 rite build      # equivalent
```

Both env vars are honored. `NO_COLOR` is the cross-tool standard ([no-color.org](https://no-color.org)); `RITE_COLOR_RESET` is the rite-specific name. CI configs commonly set `NO_COLOR=1` globally, which silences color from rite, golangci-lint, and any other tool that respects the convention.

## GitHub Actions: groups

For collapsible log groups in GitHub Actions, set `output:` on a task to wrap its cmds in `::group::` / `::endgroup::` markers:

```yaml
tasks:
  ci:
    output:
      group:
        begin: "::group::${TASK}"
        end: "::endgroup::"
    cmds:
      - go vet ./...
      - go test ./...
```

```
::group::ci
[stdout from go vet and go test]
::endgroup::
```

GitHub Actions renders the wrapped output as a collapsed, click-to-expand block in the workflow log. A single template variable like <span v-pre>`${TASK}`</span> lets you reuse the same `output:` block across many tasks.

## Picking output style automatically

A common pattern: detect CI at the top of the Ritefile and pick the appropriate output mode:

```yaml
version: '3'

vars:
  GOTESTSUM_FORMAT:
    sh: '[ -n "$CI" ] && echo github-actions || echo pkgname'

tasks:
  test:
    cmds:
      - gotestsum --format=${GOTESTSUM_FORMAT} ./...
```

`$CI` is whatever your runner sets — `CI=true` is universal across GitHub Actions, GitLab CI, CircleCI, Buildkite, etc. The `sh:` form lets you branch on the env var inside the variable definition; you could also use a Go-template `if/else` inside a `vars:` value if you prefer.

## Exit codes

CI scripts care about exit codes. rite's are documented in [CLI § Exit codes](./cli#exit-codes). Two flags worth knowing for CI:

- `--exit-code` (`-x`) — exit with the failing cmd's exact code instead of rite's wrapped 200+ form. Useful when downstream CI tooling parses specific codes.
- `--fail-on-status` — exit non-zero if any named task is *not up-to-date* (per `sources:` / `generates:`). Useful for "fail the build if the generated files weren't regenerated."

## Caching the `.rite` directory

rite stores per-task fingerprints under `.rite/` (or `$RITE_TEMP_DIR`). Caching that directory across CI runs makes incremental builds work between runs:

```yaml
# GitHub Actions example
- uses: actions/cache@v4
  with:
    path: .rite
    key: rite-${{ runner.os }}-${{ hashFiles('Ritefile.yml', '**/*.go') }}
```

When the cache hits, tasks whose `sources:` haven't changed will skip — same as on a developer's machine.

## Installing rite in CI

Three straightforward options:

- **`go install`** — fastest if your runner already has Go:
  ```sh
  go install github.com/clintmod/rite/cmd/rite@v1.0.9
  ```
  The version reported by `rite --version` will be wrong (a known [`go install` quirk](https://github.com/clintmod/rite/blob/main/CLAUDE.md)) but everything works.

- **Release archive** — for a controlled binary version, download from [releases](https://github.com/clintmod/rite/releases) using the install script:
  ```sh
  curl -sSL https://raw.githubusercontent.com/clintmod/rite/main/install.sh | sh -s -- -b /usr/local/bin v1.0.9
  ```

- **Homebrew** — fine for macOS runners that already have brew:
  ```sh
  brew install clintmod/tap/rite
  ```

## GitHub Actions: error annotations

When `GITHUB_ACTIONS=true` is set in the environment (which Actions does automatically), rite emits a `::error::` annotation on task failure. The annotation surfaces in the workflow run UI as an inline error attached to the failed task name:

```
::error title=task: my-task::exit status 1
```

You don't have to enable anything — it's automatic. For non-Actions CI systems, the annotation is suppressed (it would just be noisy log line) but the exit code still propagates so the runner knows the build failed.

If you want to suppress annotations even on GitHub Actions (e.g., your workflow handles failure presentation differently), unset the env var inline:

```yaml
- name: rite
  run: GITHUB_ACTIONS= rite ci
```

## Watch mode in CI

Don't. `rite --watch` is interactive and never exits. CI runners will hang. See [watch § when not to use](./watch#when-not-to-use).
