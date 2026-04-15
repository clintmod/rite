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
- **`run: once`:** see [run modes](./syntax#commands).
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

Namespaces nest — `foo:lib:greet` is valid if `foo` includes `lib` which has a `greet` task. See [schema: `includes`](./schema#includes).

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

## Fail-fast: cancel siblings on first failure

By default, when one of a task's `deps:` fails, the others **continue running** to completion — rite collects all the results, then reports the failure. This is useful for `lint + test + build` style runs where you want to see *every* problem, not just the first one to fire.

If you want the opposite — kill the in-flight siblings the moment any dep fails — use `failfast`:

```yaml
tasks:
  default:
    deps: [dep1, dep2, dep3, dep4]
    failfast: true       # cancel everything else when any dep fails

  dep1: sleep 0.1 && echo dep1
  dep2: sleep 0.2 && echo dep2
  dep3: sleep 0.3 && echo dep3
  dep4: exit 1
```

With `failfast: true`, the moment `dep4` exits non-zero, rite cancels `dep1`, `dep2`, `dep3` (whichever are still running) and exits. Without it, all four run to completion and rite reports the failure at the end.

Same flag is available CLI-wide as `-F` / `--failfast`:

```sh
rite --failfast default
```

The CLI flag applies to every parallel execution in the run, not just one task's deps. Useful for one-off "I don't care about other failures, stop on the first one" invocations.

When to reach for failfast:
- Long-running deps where you'd rather save the seconds than see all failures (e.g., builds that take minutes).
- Deps with side effects you don't want to occur when you already know the run will fail.
- CI cost optimization.

When to leave it off (the default):
- Test/lint runs where seeing every failure helps fix the underlying problem in one go.
- Independent deps with no coupling — there's nothing to gain from canceling them.

## Once-only execution

If multiple tasks in a run depend on the same task, rite runs that dep **once** per `rite` invocation — its result is memoized for subsequent asks. Setting `run: once` on the depended-on task guarantees this even across weirder call graphs:

```yaml
tasks:
  expensive:
    run: once
    cmds: ['./generate-schema']
```
