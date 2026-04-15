# Task aliases

Alternate names for a task. Useful for short CLI invocations (`rite b` vs `rite build`) or for renaming a task without breaking callers.

## Basics

```yaml
version: '3'

tasks:
  build:
    aliases: [b, compile]
    cmds:
      - go build ./...
```

All three work:

```sh
rite build
rite b
rite compile
```

Aliases behave identically to the primary name — they show in `--list`, in `--summary`, in tab-completion, and in `task:` call sites.

## Renaming without breaking callers

When you rename a task, keep the old name as an alias for a release or two:

```yaml
tasks:
  ci:
    aliases: [test-ci]    # old name, kept for Jenkins, CI configs, muscle memory
    cmds:
      - go test ./...
```

Nobody's pipeline breaks. Drop the alias when you've had time to update the callers.

## Alias collisions

Two tasks can't share the same name *or* alias — rite rejects the Ritefile at parse time if a collision would make a name ambiguous. This includes aliases across included files (with namespacing): `lib:foo` and `lib:f` are both fine, but two aliases called `f` in the same scope are not.

## Includes

Includes themselves take aliases too:

```yaml
includes:
  mobile:
    taskfile: ./mobile
    aliases: [m, mob]

tasks:
  release:
    cmds:
      - task: mobile:build
      - task: m:sign      # same target, shorter namespace
```

The include alias is a drop-in replacement for the include's name at call sites.
