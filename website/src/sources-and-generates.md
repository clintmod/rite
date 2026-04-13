# Incremental builds (`sources:` and `generates:`)

rite can skip a task when its inputs haven't changed since its outputs were last written. This is the core of "rebuild only what changed" — the same pattern make has used for decades, but driven by file metadata rather than by explicit rules.

## Minimal example

```yaml
tasks:
  build:
    sources:
      - 'src/**/*.go'
      - go.mod
      - go.sum
    generates:
      - './bin/app'
    cmds:
      - go build -o ./bin/app ./cmd/app
```

Run `rite build` → it compiles, writes `./bin/app`, and caches a fingerprint.
Run `rite build` again → it sees `sources:` haven't changed and `./bin/app` is still there, skips the cmds, exits 0 immediately.

Touch any file matched by `sources:` → next run rebuilds.

## `method`

Three values decide how rite computes "changed":

| Value | How |
|---|---|
| `checksum` (default) | SHA-256 every source. Accurate; slower on large trees. |
| `timestamp` | Compare mtime of sources vs. mtime of generates. Fast; trips on VCS-driven mtime shuffling. |
| `none` | Always rebuild. Disables incremental. |

Set per-task:
```yaml
tasks:
  build:
    method: timestamp
    ...
```

Or globally at the root:
```yaml
version: '3'
method: checksum
```

## Glob patterns

`sources:` and `generates:` accept standard double-star globs:

```yaml
sources:
  - 'src/**/*.{go,proto}'
  - '!src/**/*_test.go'   # prefix with ! to exclude
  - './Makefile'
```

Inside a glob you can use [Go-template syntax](/syntax#go-template-var) to interpolate vars:

```yaml
tasks:
  build:
    vars:
      LANG: go
    sources:
      - "src/**/*.{{.LANG}}"
```

`${VAR}` also works, but templates are the idiomatic choice inside globs because they compose with conditionals and pipelines.

## `for: sources` and `for: generates`

Loop over the matched files, producing one command per file:

```yaml
tasks:
  lint:
    sources: ['**/*.go']
    cmds:
      - for: sources
        cmd: golangci-lint run ${ITEM}
```

Each file becomes `${ITEM}` in the templated cmd. Rename with `as:`:

```yaml
cmds:
  - for:
      var: FILES
      split: "\n"
      as: FILE
    cmd: process ${FILE}
```

## `--force`

Skip the incremental check entirely:
```
rite build --force       # rerun this task regardless of freshness
rite build --force-all   # rerun this task AND all its deps
```

## When incremental is wrong

Some commands don't map to a "compile sources → produce generates" shape:

- Smoke-testing a remote API
- Starting a dev server
- A task whose output is state in a database

For those: set `method: none` or use `status:` instead.

## `status:` — shell-expression freshness

```yaml
tasks:
  migrate:
    status:
      - psql -c "SELECT 1 FROM migrations WHERE version = '${VERSION}'" | grep -q 1
    cmds:
      - ./apply-migration ${VERSION}
```

Each entry is a shell command. If *all* of them exit 0, the task is considered up-to-date and `cmds:` are skipped. Useful when freshness isn't a file but a question you can answer in a shell.

## `--status` — dry-run the check

```
rite --status build
```

Exit 0 if the task is fresh, non-zero otherwise. Great for CI pre-checks.
