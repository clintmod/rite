# Forwarding CLI arguments

Anything after `--` on the rite command line gets forwarded to the running task as `CLI_ARGS` and `CLI_ARGS_LIST`. Use this to pass through flags to underlying tools without baking them into the Ritefile.

## Basics

```yaml
version: '3'

tasks:
  test:
    cmds:
      - go test ${CLI_ARGS} ./...
```

```sh
rite test                            # → go test ./...
rite test -- -v                      # → go test -v ./...
rite test -- -run TestFoo -v         # → go test -run TestFoo -v ./...
rite test -- '-run "^Test(Foo|Bar)$"'   # quoted args pass through unchanged
```

The `--` separator tells rite "stop parsing my flags, treat everything after this as task args."

## Two forms: string and list

| Var | Type | Use when |
|---|---|---|
| `CLI_ARGS` | One string, joined with spaces | Splatting into a shell cmd |
| `CLI_ARGS_LIST` | List of individual args | Iterating with `for:`, or feeding into a tool that needs argv-style passing |

```yaml
tasks:
  parallel-test:
    cmds:
      - for:
          var: CLI_ARGS_LIST
        cmd: go test ./internal/${ITEM}
```

```sh
rite parallel-test -- pkg-a pkg-b pkg-c
# → runs three test commands in sequence
```

## Splitting a string into args

If you have `CLI_ARGS` as a string but need to count or iterate the individual tokens, the `splitArgs` template helper splits on shell-style quoting (`"foo bar" baz` → two tokens, not three). It pipes into `len` for a count or `range` for iteration. See [syntax — template helpers](/syntax) for the Go-template form.

In practice, prefer `CLI_ARGS_LIST` over splitting `CLI_ARGS` yourself — it's already split correctly and works with `for: { var: CLI_ARGS_LIST }` directly.

## Quoting and escaping

`CLI_ARGS` is templated *into* the cmd as a string. If your args contain spaces and you splat them naked, the shell will re-split them:

```yaml
tasks:
  echo:
    cmds:
      - echo ${CLI_ARGS}        # naive — shell re-splits args
      - echo "${CLI_ARGS}"      # safer — preserves the joined string
      - ${CLI_ARGS} hello       # cmd substitution — first arg becomes the command
```

For "pass each arg through as a separate argv", reach for `CLI_ARGS_LIST` plus `for:` instead of trying to escape inside a single cmd.

## Not exported by default

`CLI_ARGS` and `CLI_ARGS_LIST` are marked `export: false` internally — they don't leak into the cmd's environment as `$CLI_ARGS`. They're template-only, accessed via `${CLI_ARGS}` in cmds. This stops accidental leakage to subprocesses that might log their environment.

If you genuinely want a child process to see them, splat into an explicit env var:

```yaml
tasks:
  invoke:
    cmds:
      - cmd: ./script.sh
        env:
          PASSED_ARGS: '${CLI_ARGS}'
```
