# Short task syntax

For tasks that are just "run one command," the full object form is noisy. rite accepts a one-line shorthand.

## The long form

```yaml
tasks:
  hello:
    cmds:
      - echo "Hello, World!"
```

## The short form

```yaml
tasks:
  hello: echo "Hello, World!"
```

Equivalent. Parsed into the same task with a single cmd.

## When the short form makes sense

- Thin wrappers around a CLI: `lint: golangci-lint run`, `fmt: go fmt ./...`.
- Aliases for complex commands kept elsewhere: `deploy: ./scripts/deploy.sh prod`.
- Rite­files where most tasks are one-liners and the visual weight of the full form dominates the signal.

## When to go back to the long form

The moment you need anything beyond a single cmd — `deps:`, `vars:`, `sources:`, `silent:`, `desc:`, a second `cmds:` entry — switch to the full form. There's no mixed mode.

```yaml
tasks:
  # Keep one-liners short
  fmt: go fmt ./...
  lint: golangci-lint run

  # Long form for anything real
  build:
    deps: [fmt, lint]
    sources: ['**/*.go']
    cmds:
      - go build ./...
```

## What gets auto-filled

A short-form task still gets the defaults you'd get if you wrote the long form:

- `silent: false`
- No `deps:`, no `vars:`
- `run:` mode defaults to the Ritefile's top-level `run:`, which is `always` unless you changed it.

You can combine short-form tasks with `run:` at the top of the Ritefile to get run-once semantics globally without touching each task.
