# Task dependencies

A task can declare other tasks it depends on. rite resolves the dependency graph before running anything, so `deps:` always finish before the dependent task's `cmds:`.

## Basics

```yaml
tasks:
  build:
    deps: [generate, lint]
    cmds:
      - go build ./...

  generate:
    cmds: ['go generate ./...']

  lint:
    cmds: ['golangci-lint run']
```

Running `rite build` runs `generate` and `lint` first (concurrently), then runs `build`.

## Dep with variables

Every `deps:` entry can be either a string (task name) or a mapping that passes vars:

```yaml
tasks:
  all:
    deps:
      - test
      - task: build
        vars:
          OS: darwin
      - task: build
        vars:
          OS: linux
```

Call-site vars land at the callee's tier 2 (CLI-equivalent) — they beat the callee's task-scope defaults but lose to shell env.

## Parallelism

`deps:` run **concurrently by default** — rite parallelizes the graph. If two of your tasks shouldn't run at the same time (e.g. both touch the same cache file), reach for:

- **Serial chain:** use `cmds: [- task: a, - task: b]` inside a task instead of `deps: [a, b]`. Cmds inside a task run sequentially.
- **`run: once`:** see [run modes](/syntax#commands).
- **`--concurrency 1`:** a CLI-level cap on total concurrent tasks.

## Cross-included deps

A task can depend on a task in an included Ritefile, via the include's namespace:

```yaml
includes:
  lib: ./lib

tasks:
  build:
    deps: ['lib:generate']
    cmds: ['go build ./...']
```

Namespaces nest — `foo:lib:greet` is valid if `foo` includes `lib` which has a `greet` task. See [schema: `includes`](/schema#includes).

## deps vs cmds: task

There are two ways to compose tasks, and they mean different things:

```yaml
tasks:
  chain:
    deps: [prep]        # prep runs BEFORE chain's cmds — may run in parallel with other deps
    cmds:
      - task: cleanup   # cleanup runs AFTER chain's cmds
```

- `deps:` → run first, concurrently with sibling deps
- `cmds: - task: x` → run inline at that point in the cmd sequence, serially

If you have `deps: [a, b]`, a and b may start simultaneously. If you have `cmds: [{task: a}, {task: b}]`, a runs to completion, then b.

## Once-only execution

If multiple tasks in a run depend on the same task, rite runs that dep **once** per `rite` invocation — its result is memoized for subsequent asks. Setting `run: once` on the depended-on task guarantees this even across weirder call graphs:

```yaml
tasks:
  expensive:
    run: once
    cmds: ['./generate-schema']
```
