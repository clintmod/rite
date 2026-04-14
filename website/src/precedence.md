# Variable precedence

The defining feature of rite. One table. Read top-to-bottom, highest priority first. The first tier that sets a variable wins — every lower tier becomes a default that's ignored.

## The eight tiers

| # | Tier | Where it's declared | Example |
|---|---|---|---|
| 1 | Shell environment | The calling shell's env | `export FOO=bar; rite …` |
| 2 | CLI invocation | Positional `KEY=value` on the rite command line | `rite build FOO=bar` |
| 3 | `rite --set` | Explicit flag form of #2, same precedence | `rite --set FOO=bar build` |
| 4 | Entrypoint dotenv | `dotenv:` at the top of the invoked Ritefile | `dotenv: ['.env']` |
| 5 | Entrypoint `vars:` | Top-level `vars:` of the invoked Ritefile | `vars:\n  FOO: bar` |
| 6 | Included file | Vars at the include site, then top of the included file | `include: {file: x, vars: {FOO: bar}}` |
| 7 | Task-scope `vars:` | `vars:` inside a single task — **defaults only** | `tasks:\n  build:\n    vars:\n      FOO: bar` |
| 8 | Built-in specials | `RITEFILE`, `TASK`, `ROOT_DIR`, `RITE_VERSION`, etc. | Implicit |

Shell environment is law. Nothing in a Ritefile can override it.

Task-scope `vars:` are defaults only. If any higher tier sets the same name, the task value is ignored — not overwritten by it.

## Worked example

```yaml
# Ritefile.yml
version: '3'
vars:
  ENV: staging
tasks:
  deploy:
    vars:
      ENV: development   # task-scope default
    cmds:
      - echo "Deploying to ${ENV}"
```

```sh
$ rite deploy
Deploying to staging                # tier 5 beats tier 7

$ ENV=prod rite deploy
Deploying to prod                   # tier 1 wins absolutely

$ rite deploy ENV=qa
Deploying to qa                     # tier 2 beats tier 5 and tier 7
```

In `go-task`, that third case would print `development` because task-scope `vars:` override CLI args.

## Scoping rules

A variable declared inside an included Ritefile cannot leak into its parent. Resolution walks the include DAG from the invoked task outward; each scope inherits from its parent but cannot mutate it.

- **Sibling tasks cannot see each other's task-scope vars.** Task-scope `vars:` are task-local.
- **Included Ritefiles cannot see each other's vars.** Each include is sandboxed.
- **Parent scope is visible to children; child scope is invisible to parents.** Strict inheritance.

## Dynamic variables

`sh:` variables are:

- **Lazily evaluated** — only resolved when a task that uses the variable runs.
- **Resolved in the fully-composed environment** — the shell invocation sees the final merged variable set.
- **Cached within one resolution, never across.** Two tasks with an identical `sh:` expression each evaluate independently if their calling environments differ. This fixes a long-standing upstream cache-leak bug.

## `env:` blocks

`env:` blocks follow the **same precedence model as `vars:`** — they're unified under one resolver. Anything you can declare in `vars:` you can declare in `env:`; the only difference is `env:` values are exported to the cmd shell environ by default (and `vars:` are too, unless marked `export: false`).

Where `env:` lives, mapped to the same eight tiers:

| Tier | Form | Example |
|---|---|---|
| 1 | Shell environment | `export DATABASE_URL=… ; rite migrate` |
| 4 | `dotenv:` files | `dotenv: ['.env.local', '.env']` at the Ritefile root |
| 5 | Top-level `env:` block | `env: { DATABASE_URL: 'sqlite:./dev.db' }` |
| 6 | Included file's `env:` | `env:` at top of an included Ritefile |
| 7 | Task-scope `env:` block | `env:` inside a single task definition |

**Shell env always wins.** A task-scope `env: { DATABASE_URL: foo }` is a *fallback default* — if `DATABASE_URL` is set in your shell, the task sees the shell value, not `foo`.

This is a deliberate inversion of `go-task`'s model. In upstream, task-scope `env:` would override your shell environment — meaning a Ritefile could silently mask the value you exported. In rite, the rule is consistent with vars: closest-to-the-user wins.

### Worked example with env

```yaml
version: '3'

env:
  DATABASE_URL: sqlite:./dev.db    # tier 5 default
  LOG_LEVEL: info

tasks:
  migrate:
    env:
      LOG_LEVEL: debug             # tier 7 — only used if no higher tier set it
    cmds:
      - psql ${DATABASE_URL} -f migrate.sql
      - echo "logged at ${LOG_LEVEL}"
```

```sh
$ rite migrate
psql sqlite:./dev.db …             # tier 5 wins
logged at debug                     # tier 7 is the highest tier setting LOG_LEVEL

$ DATABASE_URL=postgres://prod LOG_LEVEL=warn rite migrate
psql postgres://prod …              # tier 1 wins
logged at warn                      # tier 1 wins

$ rite migrate LOG_LEVEL=trace
logged at trace                     # tier 2 (CLI) beats both env: blocks
```

The `rite migrate` tool flags Taskfiles whose `env:` semantics change under this model — see the [migration guide](/migration).

### `dotenv:` precedence

A `dotenv:` file at the Ritefile root sits at **tier 4** — it loses to `--set` / CLI args (tier 2-3) and to shell env (tier 1), but wins over the inline `env:` block (tier 5). Use `.env.local` for developer overrides that should beat the project defaults but lose to anything explicitly passed on the command line.

```yaml
version: '3'

dotenv: ['.env.local', '.env']      # earlier files in the list win

env:
  DATABASE_URL: sqlite:./dev.db
```

If `.env.local` sets `DATABASE_URL=postgres://localhost`, that wins over the inline default. A developer's shell-exported `DATABASE_URL` still wins over `.env.local`.

## Why this matters

Upstream `go-task` inverts Unix convention: task-level `vars:` override CLI args and shell environment. A [decade of bugs](https://github.com/go-task/task/issues/2034) traces to this choice.

rite reverses it. Your shell env is tier 1. Your CLI args are tier 2. Internal `vars:` blocks are defaults, not mandates. See [`SPEC.md`](https://github.com/clintmod/rite/blob/main/SPEC.md) for the complete design contract.
