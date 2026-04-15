# Interactive CLI applications

Some cmds are interactive — REPLs, TUIs, `vim`, `kubectl exec -it`, `psql`. They need direct access to your terminal: a real TTY, no buffering, no log-prefixing. Mark those tasks `interactive: true`.

## Basics

```yaml
version: '3'

tasks:
  shell:
    interactive: true
    cmds:
      - psql $DATABASE_URL
```

Without `interactive: true`, rite wraps the cmd's output to add the `[shell] ` prefix and capture stdout/stderr for its own log streams. That wrapping interferes with anything that paints the screen with escape codes or expects an unbuffered tty.

## What it changes

`interactive: true` switches the task's output handling to interleaved mode:

- **No `[task-name]` prefix on output** — the cmd writes directly to your terminal.
- **No buffering** — output flows through the moment the cmd produces it.
- **Stdin works** — the cmd can read from your keyboard, not from a pipe rite's holding open.

It does *not* allocate a pseudo-tty if there isn't one. If you run rite under a CI system that doesn't give it a tty, an `interactive: true` task still won't have one — that's a property of the parent environment, not something rite can manufacture.

## When to reach for it

| Want | Set |
|---|---|
| `vim`, `nano`, `emacs` | `interactive: true` |
| `kubectl exec -it`, `docker run -it` | `interactive: true` |
| `psql`, `mysql`, `redis-cli` | `interactive: true` |
| TUI dashboards (k9s, lazygit, btop) | `interactive: true` |
| Anything that paints with ANSI escape codes | `interactive: true` |
| Plain logging cmds | leave the default — you want the prefix |

## Mixing with non-interactive cmds

A task can have both kinds:

```yaml
tasks:
  debug-pod:
    interactive: true
    cmds:
      - kubectl get pods -n prod      # output gets no prefix in interactive mode
      - kubectl exec -it $(kubectl get pods -n prod -l app=api -o name | head -1) -- /bin/bash
```

`interactive: true` is a task-level setting, not per-cmd. Either the whole task gets the interactive treatment or none of it does. If you want the prefix back for some cmds, split them into a separate task and chain via `cmds: - task: foo`.

## Concurrency caveat

Two `interactive: true` tasks running concurrently fight for the same terminal — output gets intermixed, and only one of them can read stdin. Don't put interactive tasks in `deps:` of a parent that runs them in parallel. Use sequential `cmds:` chaining instead.
