# Run modes

By default, a task runs once per `rite` invocation no matter how many other tasks depend on it. The `run:` key lets you override this.

| Value | Meaning |
|---|---|
| `always` (default) | Run every time the task is invoked, including through a dep graph |
| `once` | Run at most once per `rite` invocation, even if invoked multiple times |
| `when_changed` | Run once, then skip subsequent invocations with the same variables — cached by the resolved vars for this run |

## `always`

The default. Each call gets a fresh execution.

```yaml
tasks:
  notify:
    cmds: ['curl -X POST $WEBHOOK']
```

Every time `notify` is reached — whether as a top-level call or as a dep — the cmd runs.

## `once`

Exactly one execution per `rite` invocation, regardless of how many tasks depend on it.

```yaml
tasks:
  prep:
    run: once
    cmds: ['generate-schema']

  build:
    deps: [prep]
    cmds: ['go build']

  test:
    deps: [prep]
    cmds: ['go test ./...']
```

`rite` with `build` and `test` both requested will run `prep` only once, not twice.

## `when_changed`

Re-run only when the task's *variables* have changed compared to the last call within this invocation. Useful when the same task is called many times with different arguments:

```yaml
tasks:
  deploy:
    run: when_changed
    vars:
      SERVICE: default
    cmds:
      - echo "deploying ${SERVICE}"
```

```yaml
cmds:
  - task: deploy
    vars: {SERVICE: api}     # runs
  - task: deploy
    vars: {SERVICE: api}     # skipped — same vars
  - task: deploy
    vars: {SERVICE: web}     # runs — different vars
```

Without `when_changed`, all three invocations would execute.

## `run:` at the entrypoint level

Set a default for every task in the file:

```yaml
version: '3'
run: once          # every task defaults to once

tasks:
  a:
    cmds: ['…']    # runs at most once per invocation
  b:
    run: always    # override for this task
    cmds: ['…']
```

## Interaction with `sources:`/`status:`

`run:` decides whether to *consider* running the task; `sources:`/`status:` decide whether the task is *up-to-date* and should be skipped even if considered.

The order rite evaluates in:

1. `run:` check — is this task in a state where we'd consider running it?
2. Freshness check — `status:` entries, `sources:`/`generates:` fingerprint
3. If both gates are open: execute `cmds:`

A task with `run: always` and `sources:` still gets skipped if the sources haven't changed.
