# Migrating from go-task

This page is the doc to read **before** you run `rite --migrate`. It walks through why rite exists, what semantically changes when you switch, what stays the same, and how to roll back if you're not ready.

## Why rite exists

rite is a hard fork of [`go-task/task`](https://github.com/go-task/task) over one disagreement: **variable precedence**.

In upstream go-task, a task-level `vars:` block overrides a CLI-supplied variable, which overrides a shell-exported env var. That is the opposite of how every other Unix tool handles the same choice: `make`, `just`, and the POSIX shell itself all treat the caller's environment as authoritative. The behavior has been publicly debated in [go-task/task#2034](https://github.com/go-task/task/issues/2034) and [#2035](https://github.com/go-task/task/issues/2035), and upstream has committed to preserving the inversion in their planned redesign.

rite takes the opposite position. Under rite:

1. Shell-exported env wins.
2. CLI-supplied vars win next.
3. Ritefile entrypoint `vars:` / `env:` come after.
4. Task-scope `vars:` / `env:` are **defaults only** — they apply when no higher tier has set the name.

That single change is what this whole fork is about. See [the precedence page](/precedence) for the formal tier table and [SPEC.md §Variable Precedence](https://github.com/clintmod/rite/blob/main/SPEC.md) for the contract.

## Install alongside go-task (or replace it)

You do not have to uninstall `task` to try `rite`. They coexist cleanly — different binary names, different file discovery, different state directories.

| What          | go-task                                       | rite                                         |
|---------------|-----------------------------------------------|----------------------------------------------|
| Binary        | `task`                                        | `rite`                                       |
| File          | `Taskfile.yml` / `Taskfile.yaml` / …          | `Ritefile.yml` / `Ritefile.yaml` / …         |
| Config file   | `.taskrc.yml`                                 | `.riterc.yml`                                |
| State dir     | `.task/`                                      | `.rite/`                                     |
| Env prefix    | `TASK_*`                                      | `RITE_*`                                     |

Run `go-task` against `Taskfile.yml` and `rite` against `Ritefile.yml` in the same repo for as long as you need. rite's `.gitignore` template keeps both `.task/` and `.rite/` ignored so dual-install state doesn't leak.

Install rite:

```sh
# Homebrew
brew tap clintmod/tap
brew install rite

# mise (≥ 2026.4; older mise strips v prefixes on ubi)
mise use -g go:github.com/clintmod/rite/cmd/rite@latest

# go install
go install github.com/clintmod/rite/cmd/rite@latest
```

Verify:

```sh
rite --version
```

## The migration workflow

### 1. Run the migrate tool

```sh
rite --migrate Taskfile.yml
```

The tool writes `Ritefile.yml` in the same directory. **The original `Taskfile.yml` is never touched.** The migrate tool also rewrites:

- Include paths (`Taskfile.yml` references → `Ritefile.yml`)
- Special-var references: <span v-pre>`{{.TASKFILE}}`</span> → `${RITEFILE}`, <span v-pre>`{{.TASKFILE_DIR}}`</span> → `${RITEFILE_DIR}`, etc.
- Adjacent `.TASK` / `.TASK_DIR` references into their `.RITE_NAME` / `.RITE_TASK_DIR` aliases in a single expression

The tool emits a warning to stderr for every site where the old and new meanings diverge. The output is grep-friendly:

```
rite migrate: OVERRIDE-VAR   Taskfile.yml task "build": vars.GLOBAL is also declared at the entrypoint — under SPEC tier 7 the task value is now a default only.
rite migrate: OVERRIDE-ENV   Taskfile.yml task "build": env.NODE_ENV is also declared at the entrypoint — entrypoint wins in rite.
rite migrate: SECRET-VAR     Taskfile.yml vars.GITHUB_TOKEN: name matches a secret pattern and will auto-export to cmd shells in rite. Add `export: false` to keep it Ritefile-internal.
```

### 2. Review every warning

Each warning category calls out a behavior that silently changes. Skim the list and decide per site whether the rite behavior is what you want (usually yes — it's the reason you're switching) or whether to edit the Ritefile to preserve the old intent.

The five categories are documented below.

### 3. Run tasks through `rite` against the new file

```sh
rite -t Ritefile.yml --list
rite -t Ritefile.yml <your-task>
```

Compare outputs against `task -t Taskfile.yml <same-task>` until you're comfortable the behavior lines up where you want it and diverges where you expect it to.

### 4. Commit, then remove `Taskfile.yml`

Once the team is happy, commit `Ritefile.yml`, drop `Taskfile.yml`, and uninstall `task` (or leave it — they don't conflict). There is no compatibility shim; after the switch, `rite` is the runner.

### Rollback

`rite --migrate` is **non-destructive**. Your original `Taskfile.yml` is untouched. If you decide not to proceed:

```sh
rm Ritefile.yml
# Taskfile.yml is already untouched
```

You're back to the pre-migration state. Re-run `rite --migrate` any time to regenerate.

## The five user-visible semantic breaks

All five flow from one root change: first-in-wins precedence.

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

**Migration:** if you genuinely want the task to run with `ENV=development`, either move the value to the entrypoint, or pass it at the CLI: `rite deploy ENV=development`.

Warning code: `OVERRIDE-VAR`

### 2. Task-scope `env:` no longer override entrypoint `env:`

Same rule, applied to env blocks. Task-scope is tier 7 (defaults); entrypoint env is tier 5.

**Migration:** same as above. The `env:` YAML key still exists — it's just not special relative to `vars:` anymore.

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

Task-level `dotenv:` is tier 7; entrypoint `env:` is tier 5.

**Migration:** move the `dotenv:` declaration to the entrypoint so it lands at tier 4, above entrypoint `env:`.

Warning code: `DOTENV-ENTRY`

### 4. `vars:` auto-exports to the cmd shell

In upstream, the `vars:` block was visible only to Ritefile templating. In rite, every declared variable exports to the cmd process environ by default — `vars:` and `env:` are semantically the same concept, both exportable.

**This matters for secrets.** A `vars.GITHUB_TOKEN` used to stay Ritefile-internal; under rite it reaches every cmd shell that runs under that task.

**Migration:** add `export: false` to any var holding a secret:

```yaml
vars:
  GITHUB_TOKEN:
    value: "…"
    export: false
```

Warning code: `SECRET-VAR` — the migrate tool matches name patterns including `*_TOKEN`, `*_SECRET`, `*_KEY`, `PASSWORD*`, `API_KEY`, `PRIVATE_KEY`, `ACCESS_KEY`.

### 5. Shell env always wins over the Ritefile's `env:` block

Upstream had an `ENV_PRECEDENCE` experiment that let users choose; rite makes the choice. **Shell env is tier 1; nothing in a Ritefile can override it.**

```yaml
env:
  FOO: from-ritefile
```

```sh
$ FOO=from-shell rite my-task
# Upstream (default): "from-ritefile"
# rite: "from-shell"
```

**Migration:** if you were relying on the Ritefile to override the shell, the intent was probably "set a default for users who haven't exported one." Keep the `env:` block — when the shell hasn't set `FOO`, the Ritefile's `FOO: from-ritefile` is what reaches the task. When the shell *has* set it, the shell wins. That is what every other Unix tool does.

No warning code — not structurally detectable from the YAML alone.

## What does **not** change

Most of what you know about go-task still applies. The fork is surgical.

- **Task graph semantics.** `deps:`, `preconditions:`, `requires:`, `sources:` / `generates:`, `method: checksum|timestamp`, `run: once|always|when_changed`, `watch:`, `defer:`, `prompt:`, aliases, internal tasks, `label:`, `silent:`, `dir:`, `platforms:`, wildcards, for-loops — behavior is unchanged.
- **Shell execution.** rite still uses [`mvdan.cc/sh`](https://github.com/mvdan/sh) for `cmd:` execution; `set:` / `shopt:` still work, as does `sh: -c` interop.
- **Most variable syntax.** <span v-pre>`{{.NAME}}`</span> Go templates, `sh:` dynamic vars, `map:` vars, `ref:` vars, `enum:` requires — all unchanged. The rite-specific addition is the `${NAME}` shell-native preprocessor, which works everywhere Go templates do and is interchangeable.
- **Includes.** Local `includes:` with optional `checksum:` pinning works the same. (Remote-URL `includes:` are removed in rite 1.0 — see [CHANGELOG](https://github.com/clintmod/rite/blob/main/CHANGELOG.md). If you need cross-repo sharing, vendor the file.)
- **CLI shape.** `rite --list`, `rite --summary`, `rite --watch`, `rite --dry`, `rite --parallel`, `rite --force`, `rite -t <file>`, positional task invocation, `--` for forwarding CLI args — all match upstream. Only remote-fetch flags are gone.

If you had a custom `.taskrc.yml`, rename it to `.riterc.yml`. Every option the migrate tool found still applies (minus the `remote:` block, which is gone with the rest of the remote-Ritefile feature).

## Why not submit a PR upstream?

The variable-precedence behavior has been debated publicly in [go-task/task#2034](https://github.com/go-task/task/issues/2034) and [#2035](https://github.com/go-task/task/issues/2035). The upstream project's planned redesign preserves the inversion. rite takes the opposite position — a clean fork is the right shape for that disagreement.

We cherry-pick non-variable-system fixes from upstream (security, CI, platform support) but never merge wholesale and never PR back.
