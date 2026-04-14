---
title: Examples
description: Runnable Ritefile recipes + a migrated corpus from go-task/examples
---

# Examples

The `examples/` tree in the repo has two subtrees, each serving a different purpose.

## `recipes/` — pedagogy

Each recipe below lives in [`examples/recipes/`](https://github.com/clintmod/rite/tree/main/examples/recipes) in the repo. Every one is a complete, runnable Ritefile — clone the repo, `cd examples/recipes/<name>`, and run `rite`.

| Recipe | What it shows |
|---|---|
| [`hello/`](https://github.com/clintmod/rite/tree/main/examples/recipes/hello) | The smallest possible Ritefile and the `default:` dispatcher |
| [`variables/`](https://github.com/clintmod/rite/tree/main/examples/recipes/variables) | 8-tier variable precedence — rite's headline behavior |
| [`shell-preprocessor/`](https://github.com/clintmod/rite/tree/main/examples/recipes/shell-preprocessor) | `${VAR}` / `${VAR:-default}` / `${VAR:?required}` |
| [`includes/`](https://github.com/clintmod/rite/tree/main/examples/recipes/includes) | Namespaced sub-Ritefiles with scope isolation |
| [`dotenv/`](https://github.com/clintmod/rite/tree/main/examples/recipes/dotenv) | Entrypoint vs task-level `dotenv:` precedence |
| [`caching/`](https://github.com/clintmod/rite/tree/main/examples/recipes/caching) | `sources:` + `generates:` + `method:` re-run logic |
| [`preconditions/`](https://github.com/clintmod/rite/tree/main/examples/recipes/preconditions) | `preconditions:` (error) vs `status:` (skip) |
| [`prompt/`](https://github.com/clintmod/rite/tree/main/examples/recipes/prompt) | Destructive-task confirmation, `--yes` bypass |
| [`cross-platform/`](https://github.com/clintmod/rite/tree/main/examples/recipes/cross-platform) | `platforms:` on cmds and tasks |
| [`migrate/`](https://github.com/clintmod/rite/tree/main/examples/recipes/migrate) | Side-by-side Taskfile → Ritefile with annotated warnings |

## `migrated/` — reference corpus

Planned sibling subtree: real-world Taskfiles from [`go-task/examples`](https://github.com/go-task/examples) run through `rite --migrate` and committed as Ritefiles. Purpose is twofold — browse real-shaped Ritefiles in every size, and give CI a regression fence against future `rite --migrate` bugs. Not yet populated; tracked as a follow-up to [#101](https://github.com/clintmod/rite/issues/101).

## Why runnable examples

Reference docs tell you the shape of a feature. Examples tell you the *posture* — what a real-world use of the feature tends to look like, which corners tend to come up together, and which warnings you'll hit when you run it through `rite --migrate`.

Every recipe is intentionally small (one idea per directory). A kitchen-sink example is hard to skim; a focused one you can read in 30 seconds.

## Running a recipe

```sh
git clone https://github.com/clintmod/rite.git
cd rite/examples/recipes/variables
rite --list
rite
```

## Contributing a recipe

Small and focused beats thorough. The pattern: one directory under `examples/recipes/`, one `Ritefile.yml`, one `README.md` that explains what it demonstrates and the exact commands to run. Open a PR.
