# Shell options: `set` and `shopt`

Two YAML fields that map directly to the bash builtins of the same names. Use them to enable shell-level safety flags or non-default behaviors per task, per cmd, or for the whole Ritefile.

## `set:` — POSIX shell options

These are the flags you'd otherwise enable with `set -o pipefail`, `set -e`, `set -u`. The most useful one in a Ritefile is `pipefail`:

```yaml
version: '3'

set: [pipefail]

tasks:
  release:
    cmds:
      - go build ./... | tee build.log
```

Without `pipefail`, the cmd's exit status is the exit status of `tee` (almost always 0). With it, a `go build` failure propagates through the pipe and the task fails.

Multiple options stack:

```yaml
set: [pipefail, errexit, nounset]
```

`pipefail` is the one you actually want for catching errors. `errexit` is on by default in rite cmds; `nounset` causes more friction than it's worth in templated cmds.

## `shopt:` — bash-specific options

These mirror `shopt -s name`. The most useful: `globstar`, which makes `**` match any number of directory levels in shell globs:

```yaml
version: '3'

shopt: [globstar]

tasks:
  fmt:
    cmds:
      - gofmt -w **/*.go        # without globstar, ** = single dir
```

Other useful options: `nullglob` (unmatched globs expand to nothing instead of staying literal), `extglob` (extended glob patterns), `failglob` (unmatched globs fail the cmd).

## Three scopes

`set:` and `shopt:` can each appear in three places:

| Scope | Where | Applies to |
|---|---|---|
| Top-level Ritefile | Right under `version:` | Every cmd in every task |
| Task | Inside a task definition | Every cmd in that task |
| Cmd | Inside a single cmd entry | Just that one cmd |

Each scope is **additive** — task-level adds to global, cmd-level adds to both. There's no scope-overrides-scope behavior; the union of all three applies.

```yaml
version: '3'
set: [pipefail]                     # global
shopt: [globstar]                   # global

tasks:
  cleanup:
    set: [errexit]                  # adds to global; this task gets pipefail + errexit
    cmds:
      - cmd: rm -rf build/**/*.tmp
        shopt: [nullglob]           # adds again; this cmd gets globstar + nullglob
```

This composability is on purpose: a strict global default plus per-cmd opt-ins for niche behavior.

## When to reach for which

- **Task fails silently when a piped command errors:** add `set: [pipefail]` globally.
- **Globs that should descend into subdirectories:** add `shopt: [globstar]` to the task or globally.
- **Globs that may match nothing without failing:** add `shopt: [nullglob]` to the cmd or task.
- **Cmd uses `$UNDEFINED_VAR` and you want loud failures:** add `set: [nounset]` to the cmd. (Don't put it globally — too much friction.)

## Bash, not POSIX

`shopt` is bash-specific. If your environment uses a non-bash shell as `/bin/sh`, `shopt` options are ignored — rite invokes cmds via the shell library that supports bash-style features regardless. `set:` options are POSIX and work in any shell.
