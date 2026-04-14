# cross-platform

`platforms:` lets a cmd (or a whole task) declare which OS/arch combinations it runs on. Non-matching cmds are skipped; a task whose declared platforms don't include the host is skipped entirely.

## Run

```sh
cd examples/recipes/cross-platform
rite open-browser           # picks the right `open` command for your host
rite cpus                   # prints logical CPU count
rite linux-only             # skipped (no error) on macOS/Windows
rite arch-specific          # runs the matching darwin/linux × arm64/amd64 line
```

## What to notice

- **Cmd-level `platforms:`** — a string list on a single `cmd:` entry. Useful when most of a task is portable and only one step differs.
- **Task-level `platforms:`** — declared on the task itself. Non-matching platforms cause the task to be **skipped silently** — no error, no stderr. Good for "this task only makes sense on Linux" shapes.
- **Accepted specifiers** — `darwin`, `linux`, `windows`, `freebsd`, …; optionally with `/arch` to pin architecture (`darwin/arm64`, `linux/amd64`, `linux/arm`). The values come from Go's `runtime.GOOS` / `runtime.GOARCH`.
- **Don't hide missing-platform errors.** If a task meaningfully can't run on a platform (e.g. "deploy" on Windows when the deploy tooling is Linux-only), prefer a `preconditions:` check that errors with an explicit message over a silent task-level `platforms:` skip.

## See also

- [Platforms](https://clintmod.github.io/rite/platforms)
- `examples/recipes/preconditions/` for the "refuse to run" alternative
