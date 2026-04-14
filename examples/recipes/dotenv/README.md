# dotenv

`.env`-style files supply variables to tasks. Where the `dotenv:` declaration lives matters — entrypoint is tier 3, task-level is tier 7.

## Run

```sh
cd examples/recipes/dotenv
rite                     # → DATABASE_URL from .env, LOG_LEVEL=info
rite prod                # task-level dotenv doesn't win — see below
DATABASE_URL=override rite     # shell (tier 1) beats all dotenv sources
```

## What to notice

- **Entrypoint `dotenv:`** is tier 3. It beats entrypoint `env:` (tier 5), which is why `.env`'s `DATABASE_URL=…` wins over the `env:` block.
- **Task-level `dotenv:`** is tier 7 — defaults only. The `prod` task loads `.env.prod`, but `DATABASE_URL` was already set by the entrypoint's tier-3 dotenv, so task-level loses. This is a behavior change from upstream go-task (where task-level dotenv won); migrate tool warns about it as `DOTENV-ENTRY`.
- **Shell env (tier 1) always wins.** If the caller has `DATABASE_URL=…` exported, *no* Ritefile source can override it. This is the core rite invariant.

## How to actually switch between environments

Rather than relying on task-level dotenv, parameterize the entrypoint:

```yaml
# Move the dotenv declaration out of the task, and let the env choose the file:
dotenv: ['.env.${APP_ENV:-dev}']
```

Then `APP_ENV=prod rite` loads `.env.prod` at entrypoint (tier 3) and everything works.

## See also

- [Variable precedence](https://clintmod.github.io/rite/precedence)
- [Migration guide — DOTENV-ENTRY warning](https://clintmod.github.io/rite/migration)
