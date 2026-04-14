# migrate

Side-by-side: a small go-task `Taskfile.yml` and the `Ritefile.yml` that `rite --migrate` produces, along with the warnings it prints.

## Run

```sh
cd examples/recipes/migrate

# Generate a fresh Ritefile from the Taskfile. Note: the repo already ships
# a hand-annotated Ritefile.yml — running --migrate overwrites it.
cp Ritefile.yml Ritefile.yml.bak
rite --migrate Taskfile.yml 2> warnings.txt
cat warnings.txt
diff Ritefile.yml Ritefile.yml.bak   # differences are cosmetic (comments, spacing)
mv Ritefile.yml.bak Ritefile.yml     # restore the annotated copy
```

## Expected warnings

```
rite migrate: OVERRIDE-VAR Taskfile.yml task "build": vars.ENV is also declared at the entrypoint — under SPEC tier 7 the task value is now a default only.
rite migrate: SECRET-VAR Taskfile.yml vars.GITHUB_TOKEN: name matches a secret pattern and will auto-export to cmd shells in rite. Add `export: false` to keep it Ritefile-internal.
```

Four automated rewrites happen as well (no warnings for these — they're mechanical):

- `# yaml-language-server: $schema=https://taskfile.dev/…` → rewritten to rite's versioned schema URL.
- `{{.TASKFILE}}` / `{{.TASKFILE_DIR}}` → `${RITEFILE}` / `${RITEFILE_DIR}`.
- `{{.TASK_DIR}}` → `${RITE_TASK_DIR}` (the `.TASK` name also keeps working via a compat alias).
- `{{ .VAR }}` Go-template interpolations → `${VAR}` shell-preprocessor form (unless `--keep-go-templates` is passed).

## What each warning means and how to react

| Warning | What's happening | What to do |
|---|---|---|
| `OVERRIDE-VAR` | A task's `vars.X` has the same name as the entrypoint's `vars.X`. Under go-task the task's value won; under rite the entrypoint wins. | If the task really needed its own value, move it to a CLI arg (`rite build ENV=development`) or use a different name. |
| `OVERRIDE-ENV` | Same, but for `env:` blocks. | Same advice. |
| `DOTENV-ENTRY` | A task-level `dotenv:` file has keys that the entrypoint also defines. | Move the dotenv to the entrypoint, or rename. |
| `SECRET-VAR` | A var name matches a secret-shaped pattern (`*_TOKEN`, `API_KEY`, `PASSWORD*`, …). rite auto-exports `vars:` — this one would leak to every cmd shell. | Add `export: false` to the var, as shown in the annotated Ritefile here. |
| `TEMPLATE-KEPT` | A `{{…}}` expression used a Go-template helper (`index`, `len`, pipes, `if`/`else`) that has no clean `${VAR}` equivalent. | Left as-is. Review by hand. |

## See also

- [Migration guide](https://clintmod.github.io/rite/migration) — full precedence walkthrough and the five semantic breaks
- `examples/recipes/variables/` — the precedence inversion is the root reason `OVERRIDE-VAR` exists
