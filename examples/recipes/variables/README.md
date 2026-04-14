# variables

The headline behavior: rite's 8-tier variable precedence. This is what the whole fork is about.

## Run

```sh
cd examples/recipes/variables

# Tier 5 wins when nothing higher is set:
rite                           # → GREETING=Hello, TARGET=world

# Tier 7 (task-scope) loses to tier 5 (entrypoint):
rite override-attempt          # GREETING stays "Hello", NOT "Goodbye"

# Tier 2 (CLI) beats tier 5 (entrypoint):
rite cli GREETING=Hi           # → GREETING=Hi

# Tier 1 (shell env) beats everything:
GREETING=shell-wins rite env   # → GREETING=shell-wins
```

## What to notice

This is the single biggest behavioral difference from go-task. Under upstream, a task-scope `vars:` block *overrides* the entrypoint's `vars:`. Under rite, task-scope is **defaults only** — it applies when no higher tier has set the name.

Why: every other Unix tool (`make`, `just`, the POSIX shell) treats the caller's environment as authoritative. rite is consistent with that.

The 8 tiers (high → low):

| # | Tier | Example |
|---|---|---|
| 1 | Shell env | `FOO=1 rite` |
| 2 | CLI positional / call-site | `rite build FOO=1` |
| 3 | `dotenv:` at entrypoint | `dotenv: [.env]` at root |
| 4 | `--env` / `--var` CLI flags | (reserved) |
| 5 | **Entrypoint `vars:` / `env:`** | root-level block |
| 6 | Included Ritefile vars | `includes: { sub: {…, vars: {…} } }` |
| 7 | **Task-scope `vars:` / `env:` — defaults only** | `tasks.foo.vars:` |
| 8 | Built-in specials | `RITEFILE`, `RITE_VERSION`, … |

Full table and formal tier definitions: [precedence](https://clintmod.github.io/rite/precedence).

## See also

- [Variable precedence](https://clintmod.github.io/rite/precedence)
- [Migration guide — the five semantic breaks](https://clintmod.github.io/rite/migration)
- `examples/recipes/shell-preprocessor/` for the `${VAR}` syntax used throughout
