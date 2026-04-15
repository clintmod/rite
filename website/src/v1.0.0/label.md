# Label — override the display name

`label:` changes how a task is *shown* — in logs, in `--list`, in the prefix of its cmd output. It does not change how the task is *called*; `rite <task-name>` still uses the key under `tasks:`.

## Basics

```yaml
version: '3'

tasks:
  foo:
    label: "build & sign"
    cmds:
      - echo "hi"
```

- `rite foo` still runs it.
- The log line reads `[build & sign] echo "hi"` instead of `[foo] echo "hi"`.
- `rite --list` shows `build & sign` as the task's display name.

## Templated labels

`label:` is a Go template, evaluated against the task's variables. Handy in for-loops and matrix runs where each iteration needs a distinct label:

```yaml
tasks:
  deploy:
    cmds:
      - for: [staging, prod]
        task: deploy-one
        vars:
          ENV: '${ITEM}'

  deploy-one:
    label: "deploy → {{.ENV}}"
    cmds:
      - ./deploy.sh ${ENV}
```

Output prefixes each command with `[deploy → staging]` and `[deploy → prod]`, so interleaved concurrent output stays readable.

## When to reach for it

- **Matrix runs.** Distinguish `build (linux/amd64)` from `build (darwin/arm64)` in output.
- **Aesthetic task names.** The key under `tasks:` needs to be a valid YAML identifier; the label doesn't.
- **Third-party Ritefiles.** Include someone else's file but rename their tasks in your own output.

## Differences from go-task

Identical to upstream — `label:` is one of the unaffected-by-first-in-wins features. The template is rendered against the final merged variable set for the task, so shell env and CLI args flow through as expected.
