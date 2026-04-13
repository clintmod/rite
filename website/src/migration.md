# Migrating from go-task

rite is an intentional semantic break from `go-task`, not a drop-in replacement. There's **no compatibility shim** — you migrate, once, using `rite --migrate`, and from that point you run the new file with the new binary.

## TL;DR — run the migrate tool

```sh
rite --migrate Taskfile.yml
```

It writes `Ritefile.yml` in the same directory, rewrites include paths, and emits a warning to stderr for every site where the old and new meanings diverge. The tool's output is grep-friendly:

```
rite migrate: OVERRIDE-VAR   Taskfile.yml task "build": vars.GLOBAL is also declared at the entrypoint — under SPEC tier 7 the task value is now a default only.
rite migrate: OVERRIDE-ENV   Taskfile.yml task "build": env.NODE_ENV is also declared at the entrypoint — entrypoint wins in rite.
rite migrate: SECRET-VAR     Taskfile.yml vars.GITHUB_TOKEN: name matches a secret pattern and will auto-export to cmd shells in rite. Add `export: false` to keep it Ritefile-internal.
```

## The five user-visible breaks

Everything below comes out of one root change: first-in-wins precedence. See [the precedence page](/precedence) for the formal table.

### 1. Task-scope `vars:` no longer override entrypoint `vars:`

```yaml
# Upstream: task-scope wins → "development"
# rite:    entrypoint wins → "staging"
vars:
  ENV: staging
tasks:
  deploy:
    vars:
      ENV: development
    cmds: ['echo "$ENV"']
```

**Migration:** if you genuinely want the task to run with `ENV=development`, either move the value to entrypoint, or pass it at the CLI: `rite deploy ENV=development`.

Warning code: `OVERRIDE-VAR`

### 2. Task-scope `env:` no longer override entrypoint `env:`

Same rule, applied to env blocks. Task-scope is tier 7 (defaults); entrypoint env is tier 5.

**Migration:** same as above. The `env:` YAML key still exists — it's just not special anymore relative to `vars:`.

Warning code: `OVERRIDE-ENV`

### 3. Task-level `dotenv:` files don't override entrypoint env

```yaml
env:
  FOO: global
tasks:
  dotenv-task:
    dotenv: ['.env']   # .env has FOO=from-dotenv
    cmds: ['echo $FOO']
# Upstream: task-level dotenv wins → "from-dotenv"
# rite:    entrypoint env wins → "global"
```

Task-level dotenv is tier 7 (defaults); entrypoint env is tier 5.

**Migration:** move the dotenv declaration to the entrypoint so it lands at tier 4, above entrypoint `env:`.

Warning code: `DOTENV-ENTRY`

### 4. `vars:` auto-exports to the cmd shell

In upstream, the `vars:` block is only visible to Ritefile templating. In rite, every declared variable exports to the cmd process environ by default — `vars:` and `env:` are semantically the same concept.

**This matters for secrets.** A `vars: GITHUB_TOKEN: ...` used to stay Ritefile-internal; now it reaches every cmd shell that runs under that task.

**Migration:** add `export: false` to any var holding a secret:

```yaml
vars:
  GITHUB_TOKEN:
    value: "…"
    export: false
```

Warning code: `SECRET-VAR` (the migrate tool matches name patterns like `*_TOKEN`, `*_SECRET`, `*_KEY`, `PASSWORD*`, `API_KEY`, `PRIVATE_KEY`, `ACCESS_KEY`).

### 5. Shell env always wins over the Ritefile's `env:` block

Upstream had an `ENV_PRECEDENCE` experiment that let users choose. rite makes the choice: **shell env is tier 1; nothing in a Ritefile can override it.**

```yaml
env:
  FOO: from-ritefile
```

```sh
$ FOO=from-shell rite my-task
# Upstream (default): "from-ritefile"
# rite: "from-shell"
```

**Migration:** if you were relying on the Ritefile to override the shell, the intent was probably "set a default for users who haven't exported one." Keep the `env:` block — when the shell hasn't set `FOO`, rite's `FOO: from-ritefile` is what reaches the task. When the shell has set it, the shell wins — which is what every other Unix tool does.

No warning code (not structurally detectable — the runtime behavior just differs).

## Other changes

### File format: `Ritefile`, not `Taskfile`

rite looks for `Ritefile`, `Ritefile.yml`, `Ritefile.yaml`, `Ritefile.dist.yml`, `Ritefile.dist.yaml` (and their lowercase variants). The migrate tool writes `Ritefile.yml`.

### Binary: `rite`, not `task`

Install via:
```sh
go install github.com/clintmod/rite/cmd/rite@latest
```

### Environment variable prefix: `RITE_*`, not `TASK_*`

`RITE_COLOR_RESET`, `RITE_VERBOSE`, etc. `RITE_X_*` for experiment flags.

### Special variables renamed

| Upstream | rite |
|---|---|
| `TASKFILE` | `RITEFILE` |
| `TASKFILE_DIR` | `RITEFILE_DIR` |
| `ROOT_TASKFILE` | `ROOT_RITEFILE` |
| `TASK_VERSION` | `RITE_VERSION` |

`TASK`, `TASK_DIR`, `ALIAS`, `ROOT_DIR`, `USER_WORKING_DIR`, `RITE_EXE` unchanged.

In rite, all specials are marked `export: false` — they're visible to Ritefile templating but don't leak into cmd shell environments.

## What won't change

The runtime model is first-in-wins. It's not going to flip back. That's the whole reason this fork exists.

## Why not just submit a PR upstream?

The variable-precedence behavior has been debated publicly in [go-task/task#2034](https://github.com/go-task/task/issues/2034) and [#2035](https://github.com/go-task/task/issues/2035). The upstream project's planned redesign preserves the inversion. rite takes the opposite position — a clean fork is the right shape for that disagreement.
