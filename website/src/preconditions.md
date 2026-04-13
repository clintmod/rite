# Preconditions and `requires:`

Two different gates, for two different failure modes:

- **Preconditions** — "this task cannot run unless this shell check passes."
- **`requires:`** — "this task cannot run unless these variables are set."

## Preconditions

```yaml
tasks:
  deploy:
    preconditions:
      - sh: test -f ./secrets.env
        msg: "Run ./scripts/fetch-secrets before deploying"
      - sh: '[ "$BRANCH" = "main" ]'
        msg: "Deploy only from main"
    cmds:
      - ./deploy
```

Each entry is a shell expression. rite runs each in sequence (order matters); the first one that exits non-zero aborts the task with its `msg:`. If all pass, `cmds:` run.

**Preconditions stop the task chain.** If `deploy` has a precondition failure, no dep of `deploy` runs, and the invocation exits non-zero. Compare to `status:` which only *skips* a task if up-to-date.

## Short form

```yaml
preconditions:
  - sh: docker info
  - sh: which kubectl
```

With no `msg:`, the default message is `precondition not met: <the sh: expression>`.

## `requires:`

Preconditions handle shell checks. `requires:` handles variable presence and shape:

```yaml
tasks:
  release:
    requires:
      vars: [VERSION, REGISTRY]
    cmds:
      - docker push ${REGISTRY}/app:${VERSION}
```

If either `VERSION` or `REGISTRY` is unset when `release` runs, rite aborts with a clear error naming the missing var. Set them via any precedence tier: shell env, CLI arg, entrypoint vars, etc.

## `requires:` with enum validation

Restrict a var to a fixed set of values:

```yaml
tasks:
  deploy:
    requires:
      vars:
        - name: ENV
          enum: [dev, staging, prod]
    cmds:
      - ./deploy --env ${ENV}
```

`rite deploy ENV=qa` fails with `ENV must be one of [dev, staging, prod], got 'qa'`.

## Dynamic enum via template

```yaml
vars:
  ALLOWED_ENVS:
    sh: cat .envs.json | jq -r '.envs | join(" ")'

tasks:
  deploy:
    requires:
      vars:
        - name: ENV
          enum:
            ref: .ALLOWED_ENVS
```

The enum values are computed from a `sh:` var. Useful when the valid set lives in a file or a remote source.

## Preconditions vs `status:`

They look similar but behave differently:

| | `preconditions:` | `status:` |
|---|---|---|
| All pass → | Task continues | Task is **skipped** (already up-to-date) |
| Any fails → | Task **aborts** with an error | Task **runs** |

Use `preconditions:` when "not allowed to proceed." Use `status:` when "no need to proceed."
