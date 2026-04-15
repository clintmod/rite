# Warning prompts

`prompt:` makes a task ask for confirmation before it runs. Use it on anything destructive, expensive, or irreversible — `terraform apply`, `db:migrate`, `release:publish`.

## Basics

```yaml
version: '3'

tasks:
  destroy:
    prompt: "Tear down the production environment. Continue?"
    cmds:
      - terraform destroy -auto-approve
```

```sh
$ rite destroy
task: "Tear down the production environment. Continue?" [y/N]
```

Anything that isn't `y` or `yes` aborts the task — exit non-zero, no cmds run.

## Multiple prompts

Pass a list to require several confirmations in sequence — useful when you want the user to physically read more than one line before nuking a thing:

```yaml
tasks:
  delete-customer-data:
    prompt:
      - "This will permanently delete all rows for customer ${ID}. Continue?"
      - "There is no recovery. Have you taken a backup?"
    cmds:
      - psql -c "DELETE FROM customers WHERE id = ${ID}"
```

Each prompt has to be confirmed individually. The first `n` aborts.

## Skipping prompts in CI

Add `--yes` (or `-y`) on the CLI to assume "yes" to every prompt:

```sh
rite destroy --yes
```

This is the only way to make a prompt-gated task run unattended. Don't bake `--yes` into a script that other humans run interactively — the prompt is there for a reason.

## Templating

`prompt:` is a Go template, so the message can include the actual values you're about to operate on:

```yaml
tasks:
  drop-table:
    prompt: "About to DROP TABLE {{.TABLE}} on {{.ENV}}. Sure?"
    cmds:
      - psql -c "DROP TABLE {{.TABLE}}"
```

```sh
rite drop-table TABLE=users ENV=prod
task: "About to DROP TABLE users on prod. Sure?" [y/N]
```

This is much more useful than a generic "are you sure?" — the user sees exactly what's about to happen.

## Inside `task:` calls

A task with a `prompt:` still prompts when invoked indirectly via `cmds: - task: foo` or as a `deps:` entry. There's no implicit "called from another task means skip the prompt." If you want the prompt only for direct invocations, gate it on `${CI}` or similar with a separate task.
