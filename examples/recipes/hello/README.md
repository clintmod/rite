# hello

The minimal Ritefile. Two tasks: a `default` that runs when you type `rite`, and a `greet` that shows how a task-scope `vars:` default is used when no higher tier has set the name.

## Run

```sh
cd examples/recipes/hello
rite                     # → Hello from rite.
rite greet               # → Hello, world!
rite greet NAME=Clint    # → Hello, Clint!     (CLI tier overrides task default)
NAME=shell rite greet    # → Hello, shell!     (shell env wins over CLI and defaults)
```

## What to notice

- `rite` with no task name runs `default`.
- The `NAME=...` on the command line is a **CLI-tier var** (tier 2), which beats the task-scope default (tier 7).
- `NAME=shell rite greet` shows the signature rite behavior: **shell env (tier 1) wins over everything**, including CLI args and task defaults. See [precedence](https://clintmod.github.io/rite/precedence) for the full tier table.

## See also

- [Getting started](https://clintmod.github.io/rite/getting-started)
- [Variable precedence](https://clintmod.github.io/rite/precedence)
