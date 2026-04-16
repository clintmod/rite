# Calling another task

There are two ways one task triggers another: as a dependency (`deps:`) or as an inline cmd (`cmds: - task: foo`). They look similar and they're not the same.

## `deps:` — run before, in parallel with siblings

```yaml
tasks:
  build:
    deps: [generate, lint]
    cmds:
      - go build ./...
```

`generate` and `lint` run **first and concurrently**. Once both finish, `go build` runs. See [task dependencies](./deps) for the full deps story.

## `cmds: - task: foo` — run inline, sequentially

```yaml
tasks:
  release:
    cmds:
      - task: build
      - task: test
      - ./publish.sh
```

`build` runs to completion, then `test`, then the shell cmd. Each step blocks the next.

## Forwarding vars

Both forms accept `vars:` to pass values into the callee:

```yaml
tasks:
  build-all:
    cmds:
      - task: build
        vars:
          OS: linux
          ARCH: amd64
      - task: build
        vars:
          OS: darwin
          ARCH: arm64

  build:
    cmds:
      - GOOS=${OS} GOARCH=${ARCH} go build -o bin/app-${OS}-${ARCH} ./cmd/app
```

Call-site `vars:` land at the callee's **CLI tier (tier 2)** in the [precedence table](./precedence) — they beat the callee's task-scope defaults but lose to shell env. So:

```sh
$ rite build-all                # OS=linux/darwin from call sites
$ OS=freebsd rite build-all     # OS=freebsd for both runs (tier 1 wins)
```

This is the model: a task's `vars:` block is its set of *defaults*. Callers can override them. A user with shell env set overrides everyone.

## `silent:` per-call

Both forms accept `silent:` on the call to suppress the echo prefix without modifying the callee:

```yaml
tasks:
  release:
    cmds:
      - task: bump-version
      - task: tag
        silent: true             # don't echo the cmd, just run it
      - ./publish.sh
```

## Self-recursion

A task can call itself, but rite has no cycle detection beyond a recursion limit. If you write a task that calls itself unconditionally, it'll error out at the depth limit instead of looping forever — but that's a footgun. Use `for:` loops or explicit args to bound the work.

## When to pick which

- **`deps:`** — when the work is independent of order and could run in parallel. Sibling deps run concurrently.
- **`cmds: - task:`** — when ordering matters or the callee shares output with the caller's other cmds. Sequential, in the parent's log stream.

If you have `deps: [a, b]`, a and b may start simultaneously. If you have `cmds: [{task: a}, {task: b}]`, a runs to completion, then b. See [`deps`](./deps) for more on the distinction.
