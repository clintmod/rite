---
title: .riterc config
description: Reference for the .riterc.yml user config file.
outline: deep
---

# `.riterc` config

rite reads its own user-config file, separate from any Ritefile. It controls
CLI defaults (verbose, silent, failfast, concurrency, etc.) and enables
experiments. The file is entirely optional — rite works without one.

The name is deliberately rite-specific so it can coexist with `go-task`'s
`.taskrc.yml` in the same tree without either tool reading the other's
config. See the [migration guide](./migration#config-file) for how to port
a `.taskrc.yml` to `.riterc.yml`.

## File discovery

rite loads config from three tiers and merges them. Later tiers win on
per-key conflict (project beats home beats XDG):

1. **XDG global** — `$XDG_CONFIG_HOME/rite/riterc.yml` or
   `$XDG_CONFIG_HOME/rite/riterc.yaml`. Only read when `$XDG_CONFIG_HOME`
   is set.
2. **Home global** — `$HOME/.riterc.yml` or `$HOME/.riterc.yaml`.
   Skipped when the project is already inside `$HOME` (the walk in step
   3 will find it anyway).
3. **Project walk** — starting from the current working directory, rite
   walks upward looking for `.riterc.yml` / `.riterc.yaml`. Every file
   found along the way is merged, with the closest (deepest) file winning.

The walk stops at the filesystem root; you can set project-wide defaults
in a repo-root `.riterc.yml` and a developer-local override in a
subdirectory if you want.

> **Only the file names differ from go-task.** Config keys below are
> `verbose`, `silent`, etc. — same spelling as `.taskrc.yml`. A working
> `.taskrc.yml` renamed to `.riterc.yml` generally "just works" (remove
> any `remote:` block — rite doesn't support remote Ritefiles).

## Keys

### `version`

- **Type**: `string` (semver)
- **Description**: Pin the config file to a specific rite version range.
  Currently advisory only — rite does not error on mismatch. Reserved
  for future schema evolution.

```yaml
version: '1'
```

### `verbose`

- **Type**: `bool`
- **Default**: `false`
- **Description**: Print extra diagnostic output during task execution.
  Equivalent to passing `-v` / `--verbose` on every invocation. The
  `RITE_VERBOSE` env var and the CLI flag take precedence.

```yaml
verbose: true
```

### `silent`

- **Type**: `bool`
- **Default**: `false`
- **Description**: Suppress command echo and task headers globally.
  Equivalent to passing `-s` / `--silent`. CLI flag overrides. Task-level
  `silent:` still applies on top.

```yaml
silent: true
```

### `color`

- **Type**: `bool`
- **Default**: auto-detect (disabled when `NO_COLOR` is set, enabled
  when stdout is a TTY, etc.)
- **Description**: Force color output on or off. Equivalent to the
  `-c` / `--color=false` flag and the `RITE_COLOR` env var. `NO_COLOR`
  (universal) and `FORCE_COLOR` (universal) still win over this setting.

```yaml
color: false
```

### `disable-fuzzy`

- **Type**: `bool`
- **Default**: `false`
- **Description**: Disable fuzzy matching on task names. With fuzzy
  matching on (the default), `rite bld` will offer to run `build` if
  there's no exact match. Setting this to `true` makes rite fail loud
  on typos instead. Equivalent to `--disable-fuzzy`.

```yaml
disable-fuzzy: true
```

### `concurrency`

- **Type**: `int`
- **Default**: `0` (unlimited — parallel tasks all run at once)
- **Description**: Cap the number of tasks running in parallel. Applies
  to `deps:` and task-level parallel execution. Equivalent to the
  `-C` / `--concurrency N` flag.

```yaml
concurrency: 4
```

### `interactive`

- **Type**: `bool`
- **Default**: `false`
- **Description**: Mark every task as interactive (forcing serial
  execution and TTY attachment). Usually you want per-task
  `interactive: true` instead — see
  [Interactive cmds](./interactive). Equivalent to `--interactive`.

```yaml
interactive: true
```

### `failfast`

- **Type**: `bool`
- **Default**: `false`
- **Description**: When any parallel `for:` iteration fails, cancel the
  rest immediately instead of letting them finish. Equivalent to the
  `-F` / `--failfast` flag.

```yaml
failfast: true
```

### `experiments`

- **Type**: `map[string]int`
- **Description**: Opt into named experiments by version number. The
  value is the experiment version the config is written against — if
  the running rite doesn't support that version, it errors loudly
  rather than silently fall back.
- **Available experiments:**
  - `GENTLE_FORCE` (version `1`) — soften the behavior of `--force`.

Experiments can also be toggled via `RITE_X_<NAME>` env vars, which win
over config-file values.

```yaml
experiments:
  GENTLE_FORCE: 1
```

## Complete example

```yaml
# .riterc.yml
version: '1'

verbose: false
silent: false
color: true
disable-fuzzy: false

concurrency: 8
failfast: true
interactive: false

experiments:
  GENTLE_FORCE: 1
```

## Precedence with the CLI and env

Config-file values are the lowest-precedence layer. The full order for
any given key is:

1. CLI flag (highest)
2. `RITE_<KEY>` env var (e.g. `RITE_VERBOSE=1`)
3. Project `.riterc.yml` (deepest in the walk wins)
4. Home `$HOME/.riterc.yml`
5. XDG `$XDG_CONFIG_HOME/rite/riterc.yml` (lowest)

This matches rite's overall first-in-wins model for Ritefile variables —
see [precedence](./precedence) for the Ritefile-side rules.
