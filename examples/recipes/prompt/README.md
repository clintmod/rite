# prompt

`prompt:` asks the user to confirm before running a destructive task. `rite -y` / `--yes` bypasses the prompt — same flag, both tasks.

## Run

```sh
cd examples/recipes/prompt

rite reset-db                 # prompts, reads y/N
rite reset-db -y              # skips prompt, runs
rite rm-cache --yes           # same as above

rite safe                     # no prompt — no prompt: field on the task
```

## What to notice

- `prompt:` is a **task field**, not a cmd modifier. You declare it once at the top of the task, and it fires before any cmd runs.
- `-y` / `--yes` is the global auto-confirm. It also exposes itself as `${CLI_ASSUME_YES}` inside templates, in case a cmd wants to branch on it:

```yaml
tasks:
  nuke:
    prompt: "Really?"
    cmds:
      - cmd: rm -rf .cache
        if: '{{.CLI_ASSUME_YES}}'        # only runs under --yes
      - cmd: echo "Dry-run; pass --yes to actually remove."
        if: '{{not .CLI_ASSUME_YES}}'
```

- In CI, always pass `-y`. A blocking prompt with no TTY is a hang.

## See also

- [Prompts](https://clintmod.github.io/rite/prompt)
- [Special CLI_* variables — CLI_ASSUME_YES](https://clintmod.github.io/rite/special-vars)
