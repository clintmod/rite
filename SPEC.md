---
date: 2026-04-12
tags:
  - project
  - spec
status: draft
---

# rite — Idempotent Task Runner

> A task runner where variables behave the way Unix taught you to expect.

## Thesis

`go-task`'s variable model inverts Unix precedence: **task-level `vars:` override everything, including CLI arguments and shell environment.** The upstream project's planned redesign (go-task/task#2035) preserves this inversion. A decade of bugs and a megathread (go-task/task#2034) all trace back to the same choice.

`rite` starts from the opposite premise: **first-in-wins.** The closer a variable is declared to the user, the more authority it has. Task-level `vars:` are *defaults*, not *overrides*. Nothing a Ritefile declares internally can override what the user passed on the command line or set in their shell.

`rite` is an intentional break from go-task: new file format (`Ritefile`), new binary, no compatibility shim. It exists because go-task's design choices cannot be fixed incrementally.

---

## Core Principles

1. **First-in-wins.** The earliest-bound value wins at every level. Your shell environment is law.
2. **Defaults, not overrides.** Internal `vars:` blocks declare what a value *should be if nothing else sets it*. They never clobber caller-provided values.
3. **Pure resolution.** A variable's value is a deterministic function of its call context. No hidden caching, no cross-task leakage, no order-dependent surprises.
4. **One variable concept.** No separate `vars:` and `env:`. Everything is a variable. Everything exports to the shell. (See go-task/task#2036 — upstream proposed this and then sat on it.)
5. **Shell-native syntax.** `${VAR}` is the primary way to reference a variable. Go-template syntax stays available for conditionals and funcs in globs/commands where it earns its keep.

---

## Variable Precedence

Listed from **highest priority (wins)** to **lowest priority (default)**:

1. **Shell environment** — variables set in the calling shell. Never overridden by anything in a Ritefile.
2. **CLI invocation** — `FOO=bar rite build` or `rite build FOO=bar`. Overrides Ritefile-internal values for this invocation only.
3. **Entrypoint `.env` files** — `dotenv:` declared at the top of the entrypoint Ritefile.
4. **Entrypoint `vars:`** — top-level `vars:` of the Ritefile that was directly invoked.
5. **Included Ritefile vars** — vars passed at the include site (`include: { file: ..., vars: {...} }`), then vars declared at the top of the included file.
6. **Task-scope `vars:`** — declared inside a specific task. **These are defaults only.** If any higher tier set this variable, the task-scope value is ignored.
7. **Built-in variables** — `RITE_DIR`, `RITE_ROOT`, etc. Lowest priority so any user-set value wins.

### Worked example

```yaml
# Ritefile
version: "1"
vars:
  ENV: staging
tasks:
  deploy:
    vars:
      ENV: development   # task-scope default
    cmds:
      - echo "Deploying to ${ENV}"
```

- `rite deploy` → `Deploying to staging` (entrypoint `vars:` wins over task-scope default)
- `ENV=prod rite deploy` → `Deploying to prod` (shell env wins over everything)
- `rite deploy ENV=qa` → `Deploying to qa` (CLI wins over entrypoint and task-scope)

Compare to go-task, where case 3 would print `development` because task-scope `vars:` override CLI args.

---

## Scoping Rules

A variable set in a child (included) Ritefile **cannot leak into its parent**. Resolution walks the include DAG from the invoked task outward; each scope inherits from its parent but cannot mutate it.

- **Sibling tasks cannot see each other's task-scope vars.** Task-scope `vars:` are task-local.
- **Included Ritefiles cannot see each other's vars.** Each include is sandboxed.
- **Parent scope is visible to children; child scope is invisible to parents.** Strict inheritance.

This explicitly addresses go-task/task#2600, #2680, #2108 — all of which are symptoms of the same missing sandbox.

---

## Dynamic Variables (`sh:`)

Dynamic variables (computed by running a shell command) must be:

- **Lazily evaluated** — only resolved when a task that actually uses the variable runs. Unused dynamic vars never fire.
- **Resolved in the fully-composed environment** — the shell invocation sees the final merged variable set (all tiers applied, first-in-wins), not an intermediate snapshot.
- **Not cached across tasks by command string.** Two tasks that happen to share a `sh:` expression each evaluate it independently if their calling environments differ. (Fixes go-task's `muDynamicCache` bug.)
- **Cached within a single resolution.** A given dynamic var resolves once per `rite` invocation, not once per reference.

---

## `vars` / `env` Unification

Ritefile has a single `vars:` concept. Every declared variable:

- Is accessible to Ritefile logic (conditionals, templating, globs)
- Is exported to the process environment when a `cmds:` shell runs
- Participates in the same precedence order regardless of where it was declared

No more `vars:` vs `env:` split. No more asymmetric precedence between the two. (Addresses go-task/task#2036.)

### Non-exported variables

If a variable should be visible inside Ritefile logic but **not** exported to the shell:

```yaml
vars:
  INTERNAL_FLAG:
    value: "1"
    export: false
```

Default is `export: true`. Shorthand `FOO: bar` is equivalent to `FOO: { value: bar, export: true }`.

---

## Template Syntax

Two ways to reference a variable:

**Shell-native** (preferred for values and cmds):
```yaml
cmds:
  - docker build -t myapp:${VERSION} .
  - echo "Deploying to ${ENV}"
```

**Go-template** (for conditionals, pipelines, globs):
```yaml
sources:
  - "src/**/*.{{.LANG}}"
cmds:
  - "{{if eq .MODE \"release\"}}cargo build --release{{else}}cargo build{{end}}"
```

Both syntaxes resolve against the same variable set with identical precedence. They're interchangeable; use whichever reads cleaner per-call.

Literal `$` in a command: `$$` escapes to `$`. Literal `{{`: `{{"{{"}}`.

---

## File Format

- Filename: **`Ritefile`** (no extension) or `Ritefile.yml` / `Ritefile.yaml` (both accepted; Ritefile without extension is canonical, mirroring `Makefile`, `Justfile`, `Dockerfile`).
- YAML syntax. Schema version declared at top: `version: "3"`. rite rejects versions outside a closed range: anything below `3` ("no longer supported") and anything at or above `4` ("not supported — rite currently supports schema version 3"). A Ritefile authored against a future schema must be run by a rite that supports that schema; silent degradation to older semantics is not a mode rite offers.
- No attempt to parse `Taskfile.yml`. The `rite --migrate` flag converts go-task Taskfiles to Ritefiles, flagging constructs that don't translate cleanly (task-scope `vars:` that were relied on as overrides become explicit warnings).

---

## On-disk Paths

rite owns its own filenames end-to-end. The paths below are **SPEC-level guarantees** — they are the contract between rite and the filesystem, and coexistence with go-task in the same repo or home directory is a design goal. A project may check in a `Ritefile` alongside a `Taskfile.yml` and both tools will operate without ever reading or writing each other's files.

- **Fingerprint / cache directory:** `.rite/` in the project root. Holds per-task checksums, timestamps, and other ephemeral state rite derives from `sources:` / `generates:`. Override with `RITE_TEMP_DIR`. The directory should be `.gitignore`d. rite **never** reads or writes `.task/` — that path is go-task's and is left untouched.
- **Project config:** `.riterc.yml` or `.riterc.yaml` in the project root (or any ancestor up to `$HOME`). Controls rite-level defaults (experiments, output, color, etc.). rite **never** reads `.taskrc.yml`.
- **User-global config:** `$XDG_CONFIG_HOME/rite/riterc.yml` (falling back to `$HOME/.riterc.yml`). Merged under project config — project wins on conflict.
- **Schema / code completions:** embedded in the `rite` binary; schema also published at `clintmod.github.io/rite/schema.json` (always-latest) and `clintmod.github.io/rite/schema/v<N>.json` (pinned per schema version — pin this form in editor configs so a future v4 release doesn't invalidate v3 Ritefiles).

---

## Compatibility with go-task

**None.** This is the intentional-break option. No flag enables Taskfile mode. No drop-in binary behavior.

Rationale: every ounce of compat shim is a future maintenance tax and a semantic leak. Users who want go-task behavior can run go-task.

A `rite migrate` tool exists to ease the one-way transition.

---

## Non-goals

### Remote Ritefiles

Ritefiles must be checked into the project they build. Fetching a Ritefile over HTTP or git at run time breaks idempotency and reproducibility — a build that depends on a remote URL is not self-contained, can silently change behavior between runs, and introduces a network dependency into what should be a deterministic local workflow. It also expands the trust surface (TLS chains, proxy intercepts, supply-chain tampering) for no semantic gain that vendoring can't match.

If you want to share task definitions across repos, vendor them in: git submodule, subtree, a committed copy, or a generator script. `includes:` accepts local paths only, and any entrypoint containing `://` is rejected with a clear error.

## Out of Scope

- **Subcommands.** rite has no subcommands. The first positional argument is always a task name — `rite <task>` runs the task named `<task>`. Invocations that don't map to a user task (migrate, init, version, help) are flags (`--migrate`, `--init`, `--version`, `--help`). This keeps the task namespace uncarved: any verb a user wants to name a task is free. Precedent: `go-task` is flag-only for the same reason.
- **Bare `rite` with no `default:`.** When invoked with no positional task name and no dispatching flag, rite runs the `default:` task if one exists; otherwise it lists available tasks silently (`rite --list --silent` semantics) and exits 0. A Ritefile with zero declared tasks gets a short "no tasks defined" hint instead. Ritefile authors should not have to write a boilerplate `default: rite -l` task.
- Cross-Ritefile task graph visualization.
- Plugin system.

---

## Success Criteria

v1 ships when:

1. The eight-tier precedence model is implemented and every tier has fixture tests.
2. Scoping: a variable declared in an included Ritefile provably cannot affect its parent or siblings.
3. Dynamic vars: lazy, per-resolution caching, no cross-task pollution.
4. `rite migrate` produces a working Ritefile from any Taskfile in go-task's own `testdata/` directory (or emits an explicit "this Taskfile relies on last-in-wins and needs manual review" error).
5. Docs site at `rite.run` (or chosen domain) with a migration guide and a precedence table that can be understood in under 60 seconds.

---

## Open Questions

- Domain and org name.
- Whether to adopt go-task's `includes:` / `dotenv:` / `sources:` / `generates:` keywords verbatim for familiarity, or rename for clarity.
- Whether `vars:` at top-level should accept a list (for ordered resolution) in addition to a map.
- License — MIT (matches go-task) or something stronger?
