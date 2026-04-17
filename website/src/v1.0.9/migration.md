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

That single change is what this whole fork is about. See [the precedence page](./precedence) for the formal tier table and [SPEC.md §Variable Precedence](https://github.com/clintmod/rite/blob/main/SPEC.md) for the contract.

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
brew install clintmod/tap/rite

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
- Self-referential CLI calls inside `cmds:` (`task lint` → `rite lint`, `cd sub && task build` → `cd sub && rite build`, `$(task --version)` → `$(rite --version)`); each rewrite emits a `SELFREF-CMD` warning so the change is auditable. Closes [#128](https://github.com/clintmod/rite/issues/128).

The tool emits a warning to stderr for every site where the old and new meanings diverge. The output is grep-friendly:

```
rite migrate: OVERRIDE-VAR   Taskfile.yml task "build": vars.GLOBAL is also declared at the entrypoint — under SPEC tier 7 the task value is now a default only.
rite migrate: OVERRIDE-ENV   Taskfile.yml task "build": env.NODE_ENV is also declared at the entrypoint — entrypoint wins in rite.
rite migrate: SECRET-VAR     Taskfile.yml vars.GITHUB_TOKEN: name matches a secret pattern and will auto-export to cmd shells in rite. Add `export: false` to keep it Ritefile-internal.
```

### 2. Review every warning

Each warning category calls out a behavior that silently changes — or, for `TEMPLATE-KEPT`, a syntactic site the rewriter could not confidently rewrite. Skim the list and decide per site whether the rite behavior is what you want (usually yes — it's the reason you're switching) or whether to edit the Ritefile to preserve the old intent.

The current warning taxonomy is:

| Code | Class | Emitted when |
|---|---|---|
| `OVERRIDE-VAR` | semantic | task-scope `vars:` key is shadowed by an entrypoint key |
| `OVERRIDE-ENV` | semantic | task-scope `env:` key is shadowed by an entrypoint key |
| `DOTENV-ENTRY` | semantic | task-level `dotenv:` file collides with entrypoint env/dotenv |
| `SECRET-VAR` | safety | var name matches a secret pattern and will auto-export to cmds |
| `TEMPLATE-KEPT` | syntactic | Go-template syntax was left in place because no `${VAR}` equivalent exists |

Each is documented in [§The user-visible semantic breaks](#the-user-visible-semantic-breaks) further down this page.

> **No more `SCHEMA-URL` warning.** Earlier versions of rite emitted `SCHEMA-URL` whenever a Taskfile carried a `# yaml-language-server: $schema=…` directive pointing at go-task's schema. Migrate now rewrites that directive to rite's hosted schema automatically, so the warning is obsolete. If you're reading older docs or migrate output that mention it, you can ignore — the behavior is fixed in place, not flagged.

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

## The user-visible semantic breaks

Five user-visible semantic changes — all five flow from one root change: first-in-wins precedence. The sixth warning class (`TEMPLATE-KEPT`) isn't a semantic break; it flags Go-template syntax the rewriter couldn't safely convert to `${VAR}` form.

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

### Syntactic: `TEMPLATE-KEPT` (Go-template syntax the rewriter skipped)

Migrate rewrites simple Go-template variable references to rite-native `${VAR}` form — it catches <span v-pre>`{{ .VAR }}`</span> and the two `default`-pipe shapes (<span v-pre>`{{ .VAR | default "x" }}`</span> and <span v-pre>`{{ default .OTHER .VAR }}`</span>), both of which round-trip through rite's shell preprocessor with no semantic change for the common exported-var case.

Everything else — `if`/`range` control flow, function calls like `printf` or `index`, sprig helpers, multi-step pipes beyond `default` — is **left untouched** and reported via `TEMPLATE-KEPT` so you can review and rewrite by hand.

<pre v-pre style="white-space: pre-wrap; word-break: break-word;"><code>rite migrate: TEMPLATE-KEPT Taskfile.yml:42: kept Go-template syntax "{{if eq .MODE \"release\"}}--release{{end}}" — no equivalent ${VAR} form; review manually.
</code></pre>

Unlike the semantic warnings above, this one doesn't signal a behavior change — it signals that the syntactic modernization pass couldn't convert the expression and you're left with the upstream form. Go templates still work in rite, so ignoring the warning is safe; the warning just flags sites where the idiomatic rewrite would save you a backtick-heavy Go-template expression in exchange for a shell-native one.

**Opt-out:** `rite --migrate --keep-go-templates Taskfile.yml` suppresses the whole pass (no rewrites, no warnings). Useful when you know your Ritefile relies on Go-template semantics that shell preprocessing doesn't match.

**Caveat on `default`:** <span v-pre>`{{ .VAR | default "x" }}`</span> rewrites to `${VAR:-x}`, which resolves through the cmd shell at exec time. The shell only sees **exported** rite vars, so for a var marked `export: false`, the default fires even when rite has set the var. The Go-template form sees every rite var regardless of export. For the common case this matches; if you're relying on a non-exported var feeding a defaulted template, either add `export: true` or opt out with `--keep-go-templates`.

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
