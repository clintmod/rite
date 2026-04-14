---
title: Special CLI_* variables
description: Template variables rite populates from the rite command line
---

# Special `CLI_*` variables

rite populates six `CLI_*` variables from the command line on every invocation and exposes them to Ritefile templates. They let a task see *how* it was invoked — whether `--force` was set, whether prompts should be skipped, what came after `--`, and so on — without re-parsing argv.

All six are marked `export: false` internally: they're visible via Go-template (`.CLI_NAME`) and shell-preprocessor (`${CLI_NAME}`) forms inside Ritefile templating but don't leak into the process environ of cmd shells. If a subprocess needs one, pass it through an explicit `env:` entry.

The full set, populated in `cmd/rite/task.go`:

| Variable | Type | Mirrors | Reflects |
|---|---|---|---|
| `CLI_ARGS` | string | arguments after `--` | shell-quoted, ready for splatting into a cmd |
| `CLI_ARGS_LIST` | list | arguments after `--` | one entry per argv token |
| `CLI_FORCE` | bool | `-f` / `--force` / `--force-all` | `true` if either force flag was set |
| `CLI_SILENT` | bool | `-s` / `--silent` | suppresses command echo |
| `CLI_VERBOSE` | bool | `-v` / `--verbose` | extra logging |
| `CLI_ASSUME_YES` | bool | `-y` / `--yes` | auto-confirms `prompt:` |

## `CLI_ARGS` and `CLI_ARGS_LIST`

Covered in depth on [Forwarding CLI args](/cli-args). Short version:

```yaml
tasks:
  test:
    cmds:
      - go test ${CLI_ARGS} ./...
```

```sh
rite test -- -v -run TestFoo    # → go test -v -run TestFoo ./...
```

## `CLI_FORCE`

`true` when the user passed `-f`, `--force`, or `--force-all`. Useful for making a task behave differently when the user is explicitly overriding guardrails:

```yaml
tasks:
  deploy:
    cmds:
      - cmd: ./scripts/tag-release.sh
        if: '{{not .CLI_FORCE}}'
      - ./scripts/deploy.sh
```

Without `--force`, the task tags the release first; with `--force`, it skips the tagging step and redeploys whatever's already there.

## `CLI_SILENT`

`true` when the user passed `-s` / `--silent`. Rite already suppresses command echo when this is set; use `.CLI_SILENT` only if your task produces output beyond what `silent:` on the cmd itself handles:

```yaml
tasks:
  build:
    cmds:
      - cmd: echo "building..."
        if: '{{not .CLI_SILENT}}'
      - go build ./...
```

## `CLI_VERBOSE`

`true` when the user passed `-v` / `--verbose`. Gate diagnostic output on it so normal runs stay quiet:

```yaml
tasks:
  release:
    cmds:
      - cmd: echo "resolved version: {{.VERSION}}, tag: {{.TAG}}"
        if: '{{.CLI_VERBOSE}}'
      - goreleaser release
```

## `CLI_ASSUME_YES`

`true` when the user passed `-y` / `--yes`. This is the same signal rite's built-in `prompt:` handler uses to auto-confirm. Read it yourself only when you have custom confirmation logic outside of `prompt:`:

```yaml
tasks:
  wipe-cache:
    cmds:
      - cmd: read -p "Really? [y/N] " ans && [ "$ans" = "y" ]
        if: '{{not .CLI_ASSUME_YES}}'
      - rm -rf .cache
```

See [Warning prompts](/prompt) for the built-in `prompt:` field, which reads `CLI_ASSUME_YES` automatically.

## Go-template vs `${}` form

All six are accessible in both forms:

```yaml
cmds:
  - echo {{.CLI_ARGS}}      # Go-template form
  - echo ${CLI_ARGS}        # shell-preprocessor form — same value
```

They resolve against the same variable set at the same precedence tier. Pick whichever reads cleaner for the surrounding cmd.
