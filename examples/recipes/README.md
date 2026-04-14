# rite — runnable recipes

Hand-crafted pedagogical recipes — one idea per directory. Each subdirectory is a complete, runnable Ritefile. `cd` into one and run `rite` with no extra setup.

For the real-world corpus migrated from upstream `go-task/examples`, see the sibling [`../migrated/`](../migrated/) tree.

| Recipe | What it shows |
|---|---|
| [`hello/`](./hello/) | The smallest possible Ritefile and the `default:` dispatcher |
| [`variables/`](./variables/) | 8-tier variable precedence — rite's headline behavior |
| [`shell-preprocessor/`](./shell-preprocessor/) | `${VAR}` / `${VAR:-default}` / `${VAR:?required}` |
| [`includes/`](./includes/) | Namespaced sub-Ritefiles with scope isolation |
| [`dotenv/`](./dotenv/) | Entrypoint vs task-level `dotenv:` precedence |
| [`caching/`](./caching/) | `sources:` + `generates:` + `method:` re-run logic |
| [`preconditions/`](./preconditions/) | `preconditions:` (error) vs `status:` (skip) |
| [`prompt/`](./prompt/) | Destructive-task confirmation, `--yes` bypass |
| [`cross-platform/`](./cross-platform/) | `platforms:` on cmds and tasks |
| [`migrate/`](./migrate/) | Side-by-side Taskfile → Ritefile with annotated warnings |

## Design rules these follow

- **Every recipe runs.** One `rite` invocation is enough, no external setup.
- **One idea per directory.** Recipes are small on purpose.
- **`${VAR}` form preferred** over `{{.VAR}}` — shell-native reads better for commands.
- **No secrets, no network.** Everything is `echo`-shaped. The `migrate/` recipe uses an obviously-fake token to demonstrate the `SECRET-VAR` warning.

## CI smoke

Every `examples/recipes/*/Ritefile.yml` is parsed by `rite -l` in CI (not executed — that would run user commands). The check lives in the top-level `Ritefile.yml` under `examples:verify`.

## Contributing

New recipes welcome. Keep them tiny. Before opening a PR:

```sh
rite examples:verify    # from the repo root — parses every example
```
