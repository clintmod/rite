---
title: Special variables
description: Template variables rite populates automatically on every invocation
---

# Special variables

rite populates a set of variables automatically on every invocation and exposes them to Ritefile templates. They split into two groups:

- **`CLI_*` flag mirrors** — what the user typed on the command line (`--force`, `--silent`, argv after `--`, …).
- **Runtime-context vars** — where the Ritefile lives, which task is running, what version of rite is executing.

Plus a set of **legacy go-task aliases** that keep hand-authored or un-migrated Ritefiles rendering correct values.

All special vars are accessible in both Go-template (<span v-pre>`{{.NAME}}`</span>) and shell-preprocessor (`${NAME}`) form; they resolve against the same value. The `CLI_*` subset is additionally marked `export: false` so it never leaks into cmd-shell environments.

## The full set

Source of truth: `internal/task/compiler.go` (`getSpecialVars`) and `cmd/rite/task.go` (`CLI_*` block).

| Variable | Type | What it is |
|---|---|---|
| **CLI flag mirrors** | | |
| `CLI_ARGS` | string | argv after `--`, shell-quoted for splatting |
| `CLI_ARGS_LIST` | list | argv after `--`, one entry per token |
| `CLI_FORCE` | bool | `-f` / `--force` / `--force-all` |
| `CLI_SILENT` | bool | `-s` / `--silent` |
| `CLI_VERBOSE` | bool | `-v` / `--verbose` |
| `CLI_ASSUME_YES` | bool | `-y` / `--yes` |
| **Runtime context** | | |
| `RITE_EXE` | string | path to the running `rite` binary (`os.Args[0]`, slash-normalized) |
| `RITE_VERSION` | string | rite's version (e.g. `v1.0.9`, or the local-fallback from `internal/version/version.txt`) |
| `RITE_NAME` | string | current task name (empty outside a task context) |
| `RITE_TASK_DIR` | string | absolute path to the task's working dir (`dir:` resolved against the entrypoint) |
| `RITEFILE` | string | absolute path to the Ritefile that declared the current task |
| `RITEFILE_DIR` | string | directory of `RITEFILE` |
| `ROOT_RITEFILE` | string | absolute path to the entrypoint Ritefile (the file `rite` discovered / was pointed at) |
| `ROOT_DIR` | string | directory of the entrypoint Ritefile |
| `USER_WORKING_DIR` | string | absolute path to the directory `rite` was invoked from |
| `ALIAS` | string | the alias the user typed, if the task was invoked via an alias; otherwise empty |
| **Legacy go-task aliases** (see below) | | |
| `TASK`, `TASK_DIR`, `TASKFILE`, `TASKFILE_DIR`, `ROOT_TASKFILE`, `TASK_VERSION` | — | safety-net aliases for the upstream names |

## `CLI_*` — flag mirrors

All six `CLI_*` vars let a task see *how* it was invoked — whether `--force` was set, whether prompts should be skipped, what came after `--` — without re-parsing argv. They're marked `export: false` internally, so they're visible in Ritefile templating but don't leak into the process environ of cmd shells. If a subprocess needs one, pass it through an explicit `env:` entry.

### `CLI_ARGS` and `CLI_ARGS_LIST`

Covered in depth on [Forwarding CLI args](./cli-args). Short version:

```yaml
tasks:
  test:
    cmds:
      - go test ${CLI_ARGS} ./...
```

```sh
rite test -- -v -run TestFoo    # → go test -v -run TestFoo ./...
```

### `CLI_FORCE`

`true` when the user passed `-f`, `--force`, or `--force-all`. Useful for making a task behave differently when the user is explicitly overriding guardrails:

```yaml
tasks:
  deploy:
    cmds:
      - cmd: ./scripts/tag-release.sh
        if: '[ "${CLI_FORCE}" != "true" ]'
      - ./scripts/deploy.sh
```

Without `--force`, the task tags the release first; with `--force`, it skips the tagging step and redeploys whatever's already there.

### `CLI_SILENT`

`true` when the user passed `-s` / `--silent`. Rite already suppresses command echo when this is set; use `.CLI_SILENT` only if your task produces output beyond what `silent:` on the cmd itself handles:

```yaml
tasks:
  build:
    cmds:
      - cmd: echo "building..."
        if: '[ "${CLI_SILENT}" != "true" ]'
      - go build ./...
```

### `CLI_VERBOSE`

`true` when the user passed `-v` / `--verbose`. Gate diagnostic output on it so normal runs stay quiet:

```yaml
tasks:
  release:
    cmds:
      - cmd: echo "resolved version: ${VERSION}, tag: ${TAG}"
        if: '[ "${CLI_VERBOSE}" = "true" ]'
      - goreleaser release
```

### `CLI_ASSUME_YES`

`true` when the user passed `-y` / `--yes`. This is the same signal rite's built-in `prompt:` handler uses to auto-confirm. Read it yourself only when you have custom confirmation logic outside of `prompt:`:

```yaml
tasks:
  wipe-cache:
    cmds:
      - cmd: read -p "Really? [y/N] " ans && [ "$ans" = "y" ]
        if: '[ "${CLI_ASSUME_YES}" != "true" ]'
      - rm -rf .cache
```

See [Warning prompts](./prompt) for the built-in `prompt:` field, which reads `CLI_ASSUME_YES` automatically.

## `RITE_*` / file-path / task-name vars — runtime context

These resolve against the call context of the currently-running task. They're ordinary (exported) variables; they leak into the cmd-shell environ like any other rite var.

### `RITE_EXE`

Absolute path to the running `rite` binary, with forward slashes on every platform. Useful for re-invoking rite from inside a cmd (for example, calling another task via `${RITE_EXE} other-task` when you need the exact binary that's running this invocation, not whatever `rite` happens to resolve on `$PATH`):

```yaml
tasks:
  outer:
    cmds:
      - ${RITE_EXE} inner
```

### `RITE_VERSION`

rite's version string. During a release build this is injected by goreleaser (`v1.0.9`); for local `go install` builds it falls back to `internal/version/version.txt` (which tracks the last shipped tag). Handy for gating behavior on rite version or stamping it into build output:

```yaml
tasks:
  build:
    cmds:
      - go build -ldflags "-X main.builtWith=${RITE_VERSION}" ./cmd/app
```

### `RITE_NAME` and `RITE_TASK_DIR`

`RITE_NAME` is the current task's name (the string that appears after `tasks:` in the Ritefile, not whatever alias the user typed — see `ALIAS` for that). `RITE_TASK_DIR` is the task's resolved working directory (`dir:` applied against the entrypoint; defaults to the directory of the Ritefile that declared the task if `dir:` is unset).

```yaml
tasks:
  build:
    dir: ./services/api
    cmds:
      - echo "${RITE_NAME} in ${RITE_TASK_DIR}"   # → build in /abs/path/services/api
```

Outside a task context (e.g. evaluating a top-level `vars:` expression), both are empty strings.

### `RITEFILE` and `RITEFILE_DIR`

Absolute path to the Ritefile that declared the current task, and its directory. When a task is declared in an included file these refer to the **included** file, not the entrypoint — which is the thing you usually want if you're resolving a path relative to the file that defined the task:

```yaml
# services/api/Ritefile.yml
tasks:
  compose:
    cmds:
      - docker compose -f ${RITEFILE_DIR}/compose.yml up
```

That path resolves against the included Ritefile, not against whatever entrypoint pulled it in.

### `ROOT_RITEFILE` and `ROOT_DIR`

Absolute path to the **entrypoint** Ritefile (the one `rite` discovered or was pointed at with `-t`), and its directory. Use these when you need to resolve paths against the top-level project root regardless of which include declared the task:

```yaml
tasks:
  tag:
    cmds:
      - git -C ${ROOT_DIR} tag ${VERSION}
```

### `USER_WORKING_DIR`

Absolute path to the directory the user ran `rite` from. This is distinct from `ROOT_DIR` (entrypoint location) and `RITE_TASK_DIR` (task `dir:`): `USER_WORKING_DIR` is where the shell was when the `rite` process started. Useful for tasks that operate on paths the user passed as arguments, since those paths were resolved relative to the user's shell.

```yaml
tasks:
  format-file:
    cmds:
      - gofmt -w ${USER_WORKING_DIR}/${CLI_ARGS}
```

### `ALIAS`

The alias the user typed, when a task was invoked via an alias declared in `aliases:`. Empty when the user invoked the task by its canonical name.

```yaml
tasks:
  deploy-production:
    aliases: [prod, production]
    cmds:
      - echo "invoked as ${ALIAS:-deploy-production}"
```

`rite prod` prints `invoked as prod`; `rite deploy-production` prints `invoked as deploy-production`.

## Legacy go-task aliases

rite keeps six variables from the upstream go-task naming scheme populated as safety-net aliases. They render the same values as their rite-prefixed counterparts so hand-authored or un-migrated Ritefiles that still reference them don't render empty strings:

| Legacy name | Rite-native equivalent |
|---|---|
| `TASK` | `RITE_NAME` |
| `TASK_DIR` | `RITE_TASK_DIR` |
| `TASKFILE` | `RITEFILE` |
| `TASKFILE_DIR` | `RITEFILE_DIR` |
| `ROOT_TASKFILE` | `ROOT_RITEFILE` |
| `TASK_VERSION` | `RITE_VERSION` |

These aliases are best-effort compatibility — the rite-prefixed names are the SPEC-preferred surface. `rite --migrate` rewrites references to them automatically when converting a Taskfile, so a migrated Ritefile shouldn't carry them forward unless you're intentionally authoring something that should stay portable back to go-task.

The aliases will keep working, but new Ritefiles should use the rite-prefixed names.

## Go-template vs `${}` form

Every special variable is accessible in both forms:

```yaml
cmds:
  - echo {{.RITEFILE}}     # Go-template form
  - echo ${RITEFILE}       # shell-preprocessor form — same value
```

They resolve against the same variable set at the same precedence tier. Pick whichever reads cleaner for the surrounding cmd. See [Syntax](./syntax) for the full rules.
