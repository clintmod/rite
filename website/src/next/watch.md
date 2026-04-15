# Watch mode

Re-run a task whenever its `sources:` change. Useful for live-reloading test runners, watching dev-server rebuilds, regenerating files when their inputs are edited.

## Basics

Two ways to invoke watch mode — CLI flag, or task field.

CLI flag (works on any task that has `sources:`):

```sh
rite --watch test
rite -w test           # short form
```

Task field (always-watch when invoked):

```yaml
version: '3'

tasks:
  test:
    watch: true
    sources: ['**/*.go']
    cmds:
      - go test ./...
```

```sh
rite test              # this enters watch mode automatically
```

The CLI flag form is usually what you want — it leaves the task itself unchanged and lets the developer choose per invocation.

## How it watches

rite uses `fsnotify` against the files matched by `sources:`. The first run executes immediately. After that, every change to a watched file re-triggers the task.

A short de-duplication window (a fraction of a second) collapses bursts of file events into one re-run — useful when an editor saves multiple files at once or a build tool writes a directory of outputs.

The interval can be tuned:

```sh
rite -w -I 500ms test       # min 500ms between re-runs
rite -w -I 2s test          # min 2s — useful for expensive tasks
```

The interval is a *minimum* gap between re-runs, not a polling interval — events still arrive via `fsnotify`, the interval just throttles how often they actually trigger a run.

## What gets watched

The `sources:` glob. Same syntax as [incremental builds](./sources-and-generates):

```yaml
tasks:
  test:
    sources:
      - '**/*.go'
      - 'testdata/**/*'
    cmds:
      - go test ./...
```

Both globs are watched. Either changing triggers a re-run.

If you don't declare `sources:`, watch mode has nothing to watch and the task runs once and exits.

## Stopping watch

Ctrl-C. The signal propagates through to the running cmd, then rite exits cleanly.

If a re-run is already in flight when a new event fires, rite cancels the in-flight run and starts a new one. If you don't want that — for tasks where partial cancellation would corrupt state — wrap the cmd in `flock` or similar at the shell level, or use `--interval` with a value larger than the task's typical run time.

## With `for:` and `deps:`

Watch mode applies to the *invoked* task. If your watch task has `deps:` or invokes other tasks via `cmds: - task: foo`, those run as part of each iteration but aren't independently watched.

```yaml
tasks:
  watch-build-and-test:
    sources: ['**/*.go']
    cmds:
      - task: build
      - task: test
```

`rite -w watch-build-and-test` watches `**/*.go`, runs `build` then `test` on each change. Neither `build` nor `test` is watched independently.

## When not to use

- **Tasks with no clear file inputs** (e.g. cron-style polling) — watch mode wants `sources:` to change.
- **Inside CI** — watch mode is interactive and never exits cleanly under a CI runner.
- **Tasks that take longer than the change rate of your edits** — you'll just be canceling and restarting forever. Better to run on demand or use a longer `--interval`.
