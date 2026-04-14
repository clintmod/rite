# includes

A root Ritefile that pulls in two sub-Ritefiles under namespaces (`backend:*`, `frontend:*`). Each include can supply its own `vars:` at the **include site**, which are visible to tasks inside the included file but don't leak back to the root or sibling includes.

## Run

```sh
cd examples/recipes/includes
rite --list              # shows backend:build, backend:test, frontend:build, …
rite                     # runs the root default, which cascades into the includes
rite backend:build       # run a single included task
rite scope               # shows what root can and can't see
```

## What to notice

- **Namespacing by default.** An included task `build` in `backend/Ritefile.yml` is invoked as `rite backend:build`, not `rite build`.
- **Scope isolation.** `COMPONENT=api` set at the include site is visible inside `backend:*` tasks but **not** in root tasks or in `frontend:*`. Variables declared inside an included Ritefile's own `vars:` block stay inside that include too.
- **Working directory.** `dir: ./backend` on the include means `backend:build` runs with CWD=`./backend`. Paths inside the include resolve relative to the include's own directory.
- **First-in-wins applies across includes.** If the root and an include both declare the same variable, the root's declaration wins — same precedence story as single-file Ritefiles.

## See also

- [Includes](https://clintmod.github.io/rite/includes)
- [Variable precedence — tier 6 (included Ritefile vars)](https://clintmod.github.io/rite/precedence)
