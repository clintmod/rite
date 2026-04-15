# Internal tasks

Some tasks exist only to be called from other tasks — setup steps, private helpers, cleanup routines. Mark those with `internal: true` and they disappear from the CLI surface.

## Basics

```yaml
version: '3'

tasks:
  build:
    cmds:
      - task: _prep
      - go build ./...

  _prep:
    internal: true
    cmds:
      - mkdir -p bin
```

`rite build` runs `_prep` first, then builds. `rite _prep` directly? Refused — internal tasks aren't callable from the command line.

## What "internal" changes

- **Hidden from `rite --list` and `rite --list-all`.** They won't show up in the listings or in tab-completion.
- **Not callable directly from the CLI.** `rite _prep` errors out.
- **Still callable from `deps:` and from `cmds: - task: …` forms.** Other tasks reach them normally.
- **Still visible to `rite --summary _prep`.** If you know the name, you can still read its docs.

## When to reach for it

The naming convention `_prefix` is enough for humans to ignore a task. `internal: true` makes the tool refuse — useful when:

- The task has destructive behavior that's only safe as part of a larger sequence.
- You're publishing a Ritefile as a library via `includes:` and want to hide the scaffolding.
- You want auto-complete to stay clean.

## Includes

A task marked `internal: true` in an included Ritefile is internal from the *importer's* perspective too — it can't be called as `lib:foo` if `foo` is internal in `lib`. That's usually what you want: the include boundary is the published surface.

If you need the opposite — private inside the file, public through the include — don't mark the task internal. Convention alone (`_helper`) is enough when the tool isn't involved.
