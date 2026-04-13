# Conditional execution: `if`

`if:` runs (or skips) a task or a single cmd based on a condition. The condition is either a shell expression (anything that exits 0/non-zero) or a Go-template expression that evaluates to "true"/"false".

## Where `if:` lives

Three places, all using the same field:

| Scope | Where | Effect |
|---|---|---|
| Task | At the top of a task definition | Skip the whole task (cmds, deps) when false |
| Cmd | Inside a single cmd entry | Skip just that one cmd, continue with the next |
| `for:` loop | On a `for:` cmd | Skip iterations whose condition is false |

The cmd is *skipped*, not failed — the task continues normally.

## Shell-expression form

The simplest `if:` is a shell test that exits 0 (truthy) or non-zero (falsy):

```yaml
tasks:
  deploy:
    cmds:
      - cmd: ./deploy-staging.sh
        if: '[ "${ENV}" = "staging" ]'
      - cmd: ./deploy-prod.sh
        if: '[ "${ENV}" = "prod" ]'
```

Either runs depending on `${ENV}`. If neither matches, both skip silently and the task succeeds with nothing done.

The `[ … ]` POSIX test, `[[ … ]]` bash test, `test …`, or any cmd that returns an exit code all work. Quote the whole `if:` value in YAML to avoid escape headaches.

## Go-template form

For conditions on rite-side variables (without dropping into shell quoting), use Go-template form. The expression must evaluate to the literal string `"true"` for the cmd to run.

The Go-template helpers most useful for `if:` are `eq` (equal), `ne` (not equal), `and`, `or`, `not`. For example, an `if:` value of <span v-pre>`'{{ eq .ENV "prod" }}'`</span> runs the cmd only when the rite-side `ENV` variable equals the string `"prod"`. Helpers compose: <span v-pre>`'{{ and (eq .ENV "prod") (eq .FEATURE_ENABLED "true") }}'`</span> requires both. A bare boolean value works too — <span v-pre>`'{{ .FEATURE_ENABLED }}'`</span> runs the cmd when the variable's string value is the literal `"true"`.

A worked example using template form lives in the test fixture at [`testdata/if/Ritefile.yml`](https://github.com/clintmod/rite/blob/main/testdata/if/Ritefile.yml) — copy that pattern when you need template conditions.

For everything below, the YAML examples use shell-expression form to keep them readable in the docs. Both forms are equivalent; pick whichever matches the variable you're branching on (`${VAR}` for shell access, template helpers for rite-side comparisons).

## Task-level `if:`

Same syntax, applied to the whole task:

```yaml
tasks:
  prod-only-cleanup:
    if: '[ "${ENV}" = "prod" ]'
    cmds:
      - ./purge-old-builds.sh
      - ./rotate-logs.sh
```

When the task is skipped, its `deps:` are not run either. (Compare to `platforms:`, which silently skips on platform mismatch — see below.)

## Inside `for:` loops

Filter loop iterations:

```yaml
tasks:
  process-files:
    cmds:
      - for: ['a.txt', 'b.txt', 'c.txt']
        cmd: ./process.sh ${ITEM}
        if: '[ "${ITEM}" != "b.txt" ]'
```

Iterations a and c run; b is skipped without an error.

## On `task:` calls

`if:` works on inline task calls too — handy for conditionally invoking sub-tasks:

```yaml
tasks:
  release:
    cmds:
      - task: build
      - task: sign
        if: '[ "${ENV}" = "prod" ]'
      - task: publish
```

In staging or dev, `sign` is skipped and the pipeline continues to `publish` (presumably with an unsigned artifact for testing). Useful when the absence of an action is acceptable, not an error.

## `if:` vs `preconditions:` vs `platforms:`

Three ways to *not* run a task. Pick based on what "not running" should mean:

| You want | Use | Behavior on miss |
|---|---|---|
| Skip silently when a context doesn't match | `if:` | Task/cmd doesn't run; task exit 0 |
| Skip silently when running on the wrong OS/arch | `platforms:` | Task/cmd doesn't run; task exit 0 |
| Fail loudly when a required check fails | `preconditions:` | Task fails with an error message |

Common confusion: `preconditions:` is for "this task SHOULD be running and would be a bug to invoke when X isn't true." `if:` is for "this task is sometimes a no-op; that's fine."

```yaml
tasks:
  release:
    preconditions:
      - sh: 'git diff-index --quiet HEAD --'
        msg: "Working tree is dirty — commit or stash first"
    if: '[ "${DRY_RUN}" != "true" ]'
    cmds:
      - ./publish.sh
```

The precondition is a hard guard — invoking `release` with a dirty tree is a bug. The `if:` is a feature flag — `DRY_RUN=true rite release` is a valid no-op invocation.

## Templating and variable precedence

Both forms (shell and template) have access to the task's full variable set, including shell env (tier 1), CLI args (tier 2), and dynamic `sh:` vars. Variable precedence applies normally — see [precedence](/precedence). A common pattern: gate prod-only behavior on a shell env var the CI sets, with a Ritefile default for local dev.

```yaml
vars:
  BRANCH: local

tasks:
  publish:
    if: '[ "${BRANCH}" = "main" ]'
    cmds:
      - ./publish.sh
```

`BRANCH=main rite publish` runs it; bare `rite publish` skips it (because tier 5 sets `BRANCH=local` as the default and tier 1 didn't override).
