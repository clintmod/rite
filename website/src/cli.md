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
| `--sort MODE` | Ordering for listings: `default`, `alphanumeric`, `none` |
| `--summary` | Long description + deps for one task (`rite --summary NAME`) |
| `-w`, `--watch` | Rerun the task when its `sources:` change |
| `-I`, `--interval DUR` | Interval for `--watch` (e.g. `500ms`, default `5s`) |
| `-v`, `--verbose` | Log var resolution, execution steps, etc. |
| `-s`, `--silent` | Suppress command echo |
| `-f`, `--force` | Force the task to run even if sources are unchanged |
| `--force-all` | With `RITE_X_GENTLE_FORCE`, force the task *and its deps* |
| `-y`, `--yes` | Auto-confirm any `prompt:` in the invoked task |
| `-n`, `--dry` | Show what would run, don't execute |
| `-x`, `--exit-code` | Pass through the failing cmd's raw exit code (see [Exit codes](#exit-codes)) |
| `--status` | Non-zero exit if any named task is not up-to-date |
| `--interactive` | Prompt for missing required vars instead of erroring |
| `--disable-fuzzy` | Turn off fuzzy matching on task names |
| `-p`, `--parallel` | Run tasks passed on the CLI in parallel |
| `-C`, `--concurrency N` | Cap concurrent task execution at N (0 = unlimited) |
| `-F`, `--failfast` | With `--parallel`, cancel siblings on first failure |
| `-c`, `--color` | Colored output (default on; use `--color=false` or `NO_COLOR=1` to disable) |
| `-o`, `--output STYLE` | Output style: `interleaved` (default), `group`, `prefixed` |
| `--output-group-begin TMPL` | Printed before a grouped task's output (requires `--output=group`) |
| `--output-group-end TMPL` | Printed after a grouped task's output (requires `--output=group`) |
| `--output-group-error-only` | Discard output from successful grouped tasks (requires `--output=group`) |
| `-d`, `--dir DIR` | Set the Ritefile's working directory |
| `-g`, `--global` | Run the global Ritefile under `$HOME` (see [File discovery](/file-discovery)) |
| `-t`, `--taskfile FILE` | Point at an explicit Ritefile |
| `--no-status` | Omit up-to-date status from `--list --json` |
| `--nested` | Nest namespaces in `--list --json` output |
| `--migrate [FILE]` | Convert a go-task Taskfile to a Ritefile. See [Migration](/migration) |
| `--keep-go-templates` | With `--migrate`, leave Go-template var refs as-is rather than rewriting to `${VAR}` |
| `--completion SHELL` | Print completion script (`bash`, `zsh`, `fish`, `powershell`) |
| `--experiments` | List experiment flags and whether each is enabled |
| `--version` | Print version |
| `-h`, `--help` | Show the full flag list in-terminal |

Full list with default values: `rite --help`.

Each of `--verbose`, `--silent`, `--force`, and `--yes` is also surfaced to task templates as a `CLI_*` bool — see [Special `CLI_*` variables](/special-vars) if a task needs to branch on which flags were passed.

## Listing and inspecting tasks

`--list` and `--list-all` produce a per-task summary suitable for `grep`-ing or piping to a fuzzy-finder.

```sh
$ rite --list
task: Available tasks for this project:
* build:        Build the binary
* test:         Run the test suite
* lint:         golangci-lint
```

`--list` shows only tasks with a `desc:` field. `--list-all` includes everything — even internal-by-convention `_helper` style tasks (but **not** tasks marked `internal: true`; those are hidden from both).

JSON form for tooling:

```sh
rite --list --json
rite --list-all --json
```

The JSON is stable enough to script against — task name, description, aliases, location. `--no-status` skips the per-task up-to-date check (useful in large repos where `status:` scripts are expensive). `--nested` groups namespaces hierarchically instead of flat-listing them.

### `--summary` for a single task

`--summary <task>` prints the long description (the task's `summary:` field, falling back to `desc:` if no `summary:` is set), plus its declared dependencies:

```sh
$ rite --summary deploy
task: deploy

Deploy the current build to the named environment. Requires
ENV=staging|prod. Runs build and test as deps before deploying.

dependencies:
 - build
 - test
```

Use `summary:` for tasks where the user really needs to know *what's about to happen* before they invoke. Use `desc:` for the one-line listing.

### Tab completion

`rite --completion bash > /etc/bash_completion.d/rite` (or zsh/fish/powershell) installs a completion script that knows your task names and flags. The script reads tasks at completion-time, so changes to the Ritefile show up immediately without re-installing.

## Passing variables

Two ways, in precedence order:

```sh
FOO=bar rite build             # 1. Shell env — tier 1 (wins over everything)
rite build FOO=bar             # 2. CLI positional — tier 2
```

See [Variable precedence](/precedence) for the full tier list.

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

`CLI_ARGS` (and the other `CLI_*` specials) are marked non-export, so they're visible inside Ritefile templating but don't leak into the process environ of cmd shells. For the full list — `CLI_ARGS`, `CLI_ARGS_LIST`, `CLI_FORCE`, `CLI_SILENT`, `CLI_VERBOSE`, `CLI_ASSUME_YES` — see [Special `CLI_*` variables](/special-vars).

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Generic error (no more specific code applies) |
| `50` | `.riterc.yml` referenced but not found |
| `100`–`106` | Ritefile parse/format errors (not found, already exists, decode, version check, invalid, cycle, checksum mismatch) |
| `200`–`207` | Task-runtime errors (not found, run error, internal, name conflict, called too many times, cancelled, missing required vars, disallowed vars) |

The exact per-code mapping lives in [`errors/errors.go`](https://github.com/clintmod/rite/blob/main/errors/errors.go).

When a cmd inside a task exits non-zero, rite's default exit code is `201` (`CodeTaskRunError`) — a fixed value, not the wrapped cmd's code. Pass `-x` / `--exit-code` to have rite pass through the failing cmd's actual exit status instead.

## Environment variables rite reads

All flag-equivalents read their value from a `RITE_<NAME>` env var if the flag wasn't set on the CLI. Precedence is: CLI flag > env > [`.riterc.yml`](/migration#riterc) > default.

| Var | Flag equivalent | Purpose |
|---|---|---|
| `RITE_VERBOSE` | `-v`, `--verbose` | Extra logging |
| `RITE_SILENT` | `-s`, `--silent` | Suppress command echo |
| `RITE_DRY` | `-n`, `--dry` | Show what would run, don't execute |
| `RITE_ASSUME_YES` | `-y`, `--yes` | Auto-confirm `prompt:` |
| `RITE_INTERACTIVE` | `--interactive` | Prompt for missing required vars |
| `RITE_DISABLE_FUZZY` | `--disable-fuzzy` | Turn off fuzzy task-name matching |
| `RITE_COLOR` | `-c`, `--color` | Enable (`1`/`true`) or disable (`0`/`false`) ANSI color |
| `RITE_CONCURRENCY` | `-C`, `--concurrency` | Max concurrent tasks |
| `RITE_FAILFAST` | `-F`, `--failfast` | Cancel siblings on first failure in parallel mode |
| `RITE_TEMP_DIR` | *(no flag)* | Override the `.rite` cache directory |
| `RITE_CORE_UTILS` | *(no flag)* | Force-enable (`true`) or disable (`false`) rite's built-in coreutils shims |
| `RITE_X_*` | *(no flag)* | Enable experiment flags (e.g. `RITE_X_GENTLE_FORCE=1`). See `rite --experiments`. |
| `NO_COLOR` | *(no flag)* | Disable ANSI color output (if `RITE_COLOR`/`--color` are not set) |
| `FORCE_COLOR` | *(no flag)* | Force ANSI color on even without a TTY (if nothing else is set) |
| `CI` | *(no flag)* | When truthy, rite behaves like `FORCE_COLOR` for CI annotation output |

Task-declared variables reach cmd shells through the process environ by default (see [syntax §Non-exported](/syntax#non-exported-variables)).
