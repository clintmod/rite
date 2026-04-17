# Silent, dry-run, and ignore-error

Three small flags that change *how* cmds run without changing what they do. They show up in different places — task body, top-level Ritefile, CLI — and they compose.

## `silent:` — suppress the echo

By default, every cmd is echoed to stderr before it runs (`task: [build] go build ./...`). `silent: true` suppresses that prefix line. The cmd's own stdout/stderr still flow through.

```yaml
tasks:
  greet:
    silent: true
    cmds:
      - echo "hi"
```

```sh
$ rite greet
hi
```

Without `silent:`, you'd also see `task: [greet] echo "hi"` first.

Per-cmd granularity:

```yaml
tasks:
  build:
    cmds:
      - cmd: rm -rf bin
        silent: true       # don't echo the cleanup
      - go build -o bin ./...
```

CLI override: `rite --silent <task>` (or `-s`) silences everything for the run.

Top-level: `silent: true` at the root of the Ritefile makes silence the default for every task.

## `--dry` — preview without running

```sh
rite --dry build
```

Prints every cmd that *would* run, doesn't execute any of them. Useful for understanding what a complex dependency graph will actually do, or for safe diffs against a CI pipeline.

`--dry` is CLI-only — there's no `dry:` field. It applies to the whole invocation.

Caveats:
- `sh:` dynamic vars **do** evaluate during dry-run (they have to, so the cmd templates can render).
- `preconditions:` are skipped — dry-run shouldn't fail because a flag-checking cmd legitimately exits 1.
- `defer:` cmds are printed.

## `ignore_error:` — keep going on failure

By default, the first cmd that exits non-zero stops the task. `ignore_error: true` flips that for one cmd or the whole task:

```yaml
tasks:
  task-should-pass:
    ignore_error: true     # task-level: any cmd can fail
    cmds:
      - exit 1
      - echo "this still runs"

  cleanup:
    cmds:
      - cmd: rm /tmp/may-not-exist
        ignore_error: true     # cmd-level: only this one
      - echo "always reaches here"
```

This is for cmds whose failure is informational — `rm` of an optional file, a status check that you want logged but not fatal. It is **not** a substitute for proper error handling in your scripts.

## How they compose

| Flag | Where | What wins |
|---|---|---|
| `silent` | top-level / task / cmd / `--silent` | Most-specific wins; CLI flag forces silence regardless of file settings |
| `--dry` | CLI only | Run-wide |
| `ignore_error` | task / cmd | Cmd-level overrides task-level |

`--dry` plus `--silent` together: prints the cmds that would run, with no per-cmd echo prefix. Closest thing to a "give me the rendered command list" dump.

## Differences from go-task

Identical to upstream — none of these flags interact with the variable system, so first-in-wins doesn't apply. The one footgun shared with upstream: `--dry` evaluating `sh:` vars means a dynamic-var that actually mutates state (writes a file, makes an HTTP call) will still run during dry-run. Treat `sh:` as if it always executes.
