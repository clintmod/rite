# preconditions

Two different "don't run the cmds" gates:

- **`preconditions:`** — a list of shell checks + messages. If any fails, the task **errors out** with the message. Use this for "refuse to proceed" guards (missing credentials, wrong OS, dirty checkout).
- **`status:`** — also shell checks, but failure means "skip silently, consider the task already done." Use this for idempotent work whose side-effect can be observed (a file exists, a service is running).

## Run

```sh
cd examples/recipes/preconditions

# Missing DEPLOY_TOKEN — task errors with the message you wrote
rite deploy
# → rite: DEPLOY_TOKEN must be set before deploying
# → rite: Failed to run task "deploy": rite: precondition not met

# With the var set — preconditions pass, cmds run
DEPLOY_TOKEN=abc123 rite deploy

# status: — first run does the work
rite fetch-data          # → wrote data/cached.txt
rite fetch-data          # → rite: Task "fetch-data" is up to date   (skipped)

rite clean               # removes data/
rite fetch-data          # runs again
```

## What to notice

- `preconditions:` **errors** on failure (exit code 201). `status:` **skips** on failure — no error, the task is considered up-to-date.
- Both accept a list, and all items must pass for the task to proceed (preconditions) or be considered up-to-date (status).
- The `msg:` on a precondition is what the user sees — write it as a clear remediation instruction, not just a restatement of the check.
- `status:` and `sources:`/`generates:` can coexist; rite checks them in that order — a matching `status:` wins, the fingerprint check is skipped.

## See also

- [Preconditions & requires](https://clintmod.github.io/rite/preconditions)
- `examples/recipes/caching/` for the `sources:`/`generates:` alternative to `status:`
