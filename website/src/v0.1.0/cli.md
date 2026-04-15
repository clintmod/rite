# CLI reference

```
rite [flags...] [task...] [KEY=value...]
```

Runs the named task(s), passing any `KEY=value` pairs as CLI-tier vars (tier 2). With no task, runs `default`.

## Most-used flags

| Flag | What it does |
|---|---|
| `-i`, `--init` | Write a starter `Ritefile.yml` in the current dir |
| `-l`, `--list` | List tasks with descriptions |
| `-a`, `--list-all` | List all tasks (including undescribed) |
| `-j`, `--json` | JSON output for `--list`/`--list-all` |
| `-w`, `--watch` | Rerun the task when its `sources:` change |
| `-v`, `--verbose` | Log var resolution, execution steps, etc. |
| `-s`, `--silent` | Suppress command echo |
| `-f`, `--force` | Force the task to run even if sources are unchanged |
| `-d`, `--dir DIR` | Set the Ritefile's working directory |
| `-t`, `--taskfile FILE` | Point at an explicit Ritefile |
| `--set KEY=value` | Explicit form of the positional `KEY=value` |
| `--dry` | Show what would run, don't execute |
| `--status` | Non-zero exit if any named task is not up-to-date |
| `--migrate [FILE]` | Convert a go-task Taskfile to a Ritefile. See [Migration](./migration) |
| `--completion SHELL` | Print completion script (bash, zsh, fish, powershell) |
| `--experiments` | List experiment flags |
| `--version` | Print version |

Full list: `rite --help`.

## Passing variables

Three ways, in precedence order:

```sh
FOO=bar rite build             # 1. Shell env — tier 1 (wins over everything)
rite build FOO=bar             # 2. CLI positional — tier 2
rite --set FOO=bar build       # 3. CLI flag form — same as tier 2
```

## Passing args to the task itself

Anything after `--` is available in the task as `CLI_ARGS`:

```sh
rite run -- --dry-run --verbose
```

```yaml
tasks:
  run:
    cmds:
      - myprogram {{.CLI_ARGS}}
```

`CLI_ARGS` (and the other `CLI_*` specials) are marked non-export, so they're visible inside Ritefile templating but don't leak into the process environ of cmd shells.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Generic error (task failed, typo, etc.) |
| `100-115` | Ritefile-parse / format errors (see `errors/errors.go`) |
| `200-201` | Task-runtime errors (exit is the wrapped cmd's code + 200) |

Pass `-x` / `--exit-code` to have `rite` exit with the failing cmd's exact code instead of the 200+ wrapped form.

## Environment variables rite reads

| Var | Purpose |
|---|---|
| `RITE_COLOR_RESET` | Disable ANSI color output |
| `RITE_TEMP_DIR` | Override the `.task` cache directory |
| `RITE_REMOTE_DIR` | Override the remote-taskfile cache directory |
| `RITE_X_*` | Enable experiment flags (e.g. `RITE_X_ENV_PRECEDENCE=1`) |
| `NO_COLOR` | Same as `RITE_COLOR_RESET` |

Task-declared variables reach cmd shells through the process environ by default (see [syntax §Non-exported](./syntax#non-exported-variables)).
