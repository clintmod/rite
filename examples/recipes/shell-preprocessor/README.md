# shell-preprocessor

rite's `${VAR}` form is a thin preprocessor that runs before `mvdan.cc/sh` executes the cmd. It accepts the familiar POSIX-parameter-expansion forms — `${VAR}`, `${VAR:-default}`, `${VAR:?error}` — and resolves against the same variable set as Go templates, at the same precedence.

Prefer `${VAR}` for shell content. It's what every reader of the resulting cmd already knows.

## Run

```sh
cd examples/recipes/shell-preprocessor
rite                        # → Running in dev on port 8080
rite defaults               # → LOG_LEVEL=info, REGION=us-west-2
rite defaults LOG_LEVEL=debug
rite required API_KEY=abc123
rite required              # fails with "API_KEY must be set before running deploy"
rite mixed                  # both forms print "dev"
```

## What to notice

- `${VAR}` and `{{.VAR}}` resolve against the **same** variable set at the **same** tier. Pick whichever reads cleaner in context.
- `${VAR:-default}` is useful for optional config (fallback when the user hasn't set one). The default is used when the var is unset *or* empty.
- `${VAR:?message}` hard-fails the cmd with a clear error. Prefer it to writing ad-hoc `if` guards around the first use of a required variable.
- The preprocessor runs once per cmd before `mvdan.cc/sh` sees the string, so escapes you'd use to protect a literal `${...}` in shell (`\${…}`) still work when you genuinely want the literal.

## See also

- [Syntax — shell preprocessor](https://clintmod.github.io/rite/syntax)
- `examples/recipes/variables/` for precedence — the `${VAR}` form doesn't change it
